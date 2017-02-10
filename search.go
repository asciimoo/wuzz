package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
)

// DefaultResponseBodyTitle defines the default header for the body view
const DefaultResponseBodyTitle = "Response body (F9)"

// Search performs a search on the request
// returns title, display, error
func Search(request *Request, searchString string) (string, []byte, error) {
	isBinary := !strings.Contains(request.ContentType, "text") &&
		!strings.Contains(request.ContentType, "application")

	if isBinary {
		return DefaultResponseBodyTitle + " [binary content]",
			[]byte(hex.Dump(request.RawResponseBody)), nil
	}

	if strings.Contains(request.ContentType, "text") {
		return SearchText(request, searchString)
	}

	if strings.Contains(request.ContentType, "json") {
		return SearchJSON(request, searchString)
	}

	return DefaultResponseBodyTitle + " [unknown content]", request.RawResponseBody, nil
}

// SearchText perform search for text typed response
func SearchText(request *Request, searchString string) (string, []byte, error) {
	if searchString == "" {
		return DefaultResponseBodyTitle, request.RawResponseBody, nil
	}

	searchRE, err := regexp.Compile(searchString)
	if err != nil {
		return DefaultResponseBodyTitle, []byte("Error: invalid search regexp"), nil
	}
	results := searchRE.FindAll(request.RawResponseBody, 1000)
	if len(results) == 0 {
		return "No results", []byte("Error: no results"), nil
	}
	title := fmt.Sprintf("%d results", len(results))
	output := ""
	for _, result := range results {
		output += fmt.Sprintf("-----\n%s\n", result)
	}
	return title, []byte(output), nil
}

// SearchJSON perform search for json type response.
// Returns a pretty-print json
func SearchJSON(request *Request, searchString string) (string, []byte, error) {
	var prettyJSON bytes.Buffer
	body := request.RawResponseBody
	if searchString != "" {
		if request.ParsedResponseBody == nil {
			request.ParsedResponseBody = gjson.ParseBytes(request.RawResponseBody)
		}
		result := request.ParsedResponseBody.(gjson.Result)
		if result.Type != gjson.JSON { // not json
			return DefaultResponseBodyTitle, request.RawResponseBody, nil
		}
		searchResult := result.Get(searchString)
		if searchResult.Type == gjson.Null {
			// if the search is bad, we will keep the previous result
			return "Showing previous result", nil, errors.New("bad search")
		}
		if searchResult.Type != gjson.JSON {
			return DefaultResponseBodyTitle + " [json-" + searchResult.Type.String() + "]",
				[]byte(searchResult.String()), nil
		}
		body = []byte(searchResult.String())
	}
	err := json.Indent(&prettyJSON, body, "", "  ")
	if err == nil {
		return DefaultResponseBodyTitle + " [json]", prettyJSON.Bytes(), nil
	}
	return DefaultResponseBodyTitle, request.RawResponseBody, nil
}
