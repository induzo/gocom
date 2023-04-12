package handlerwrap

import (
	"context"
	"net/http"
)

// Response is a wrapper for the response body.
type Response struct {
	Headers    map[string]string
	Body       any
	StatusCode int
}

func (hr *Response) Render(ctx context.Context, respW http.ResponseWriter, respEncoding Encoding) {
	Render(
		ctx,
		hr.Headers,
		hr.StatusCode,
		hr.Body,
		respEncoding,
		respW,
	)
}
