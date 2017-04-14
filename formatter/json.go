package formatter

import (
	"bytes"
	"errors"
	"io"

	"github.com/nwidger/jsoncolor"
	"github.com/tidwall/gjson"
)

type jsonFormatter struct {
	parsedBody gjson.Result
	TextFormatter
}

func (f *jsonFormatter) Format(writer io.Writer, data []byte) error {
	jsonFormatter := jsoncolor.NewFormatter()
	buf := bytes.NewBuffer(make([]byte, 0, len(data)))
	err := jsonFormatter.Format(buf, data)
	if err == nil {
		writer.Write(buf.Bytes())
		return nil
	}
	return errors.New("json formatter error")
}

func (f *jsonFormatter) Title() string {
	return "[json]"
}

func (f *jsonFormatter) Search(q string, body []byte) ([]string, error) {
	if q != "" {
		if f.parsedBody.Type != gjson.JSON {
			f.parsedBody = gjson.ParseBytes(body)
		}
		searchResult := f.parsedBody.Get(q)
		if searchResult.Type == gjson.Null {
			return nil, errors.New("Invalid gjson query or no results found")
		}
		if searchResult.Type != gjson.JSON {
			return []string{searchResult.String()}, nil
		}
		body = []byte(searchResult.String())
	}
	jsonFormatter := jsoncolor.NewFormatter()
	buf := bytes.NewBuffer(make([]byte, 0, len(body)))
	err := jsonFormatter.Format(buf, body)
	if err != nil {
		return nil, errors.New("Invalid results")
	}
	return []string{string(buf.Bytes())}, nil
}
