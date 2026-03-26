package request

import (
	"time"

	"github.com/hasithdealwis/wuzz/formatter"
)

// Request represents an HTTP request and its associated response data
type Request struct {
	Url             string                      // Request URL
	Method          string                      // HTTP method (GET, POST, etc.)
	GetParams       string                      // URL query parameters
	Data            string                      // Request body data
	Headers         string                      // Request headers
	ResponseHeaders string                      // Response headers
	RawResponseBody []byte                      // Raw response body bytes
	ContentType     string                      // Response content type
	Duration        time.Duration               // Request duration
	Formatter       formatter.ResponseFormatter // Response formatter
}
