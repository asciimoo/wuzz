// Package requests package defines the Request struct which encapsulates all the details of an HTTP request and its response. It also includes a function to convert a history entry into a Request object, allowing for easy retrieval and formatting of past requests and responses.
package requests

import (
	"time"

	"github.com/hasithdealwis/wuzz/config"
	"github.com/hasithdealwis/wuzz/formatter"
	"github.com/hasithdealwis/wuzz/pkgs/history"
)

type Request struct {
	URL             string
	Method          string
	GetParams       string
	Data            string
	Headers         string
	ResponseHeaders string
	RawResponseBody []byte
	ContentType     string
	Duration        time.Duration
	Formatter       formatter.ResponseFormatter
}

func EntryToRequest(entry *history.Entry, config *config.Config) *Request {
	return &Request{
		URL:             entry.URL,
		Method:          entry.Method,
		GetParams:       entry.GetParams,
		Data:            entry.Data,
		Headers:         entry.Headers,
		ResponseHeaders: entry.ResponseHeaders,
		RawResponseBody: entry.RawResponseBody,
		ContentType:     entry.ContentType,
		Duration:        entry.Duration,
		Formatter:       formatter.New(config, entry.ContentType),
	}
}
