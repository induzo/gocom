package handlerwrap

import (
	"context"
	"net/http"

	"github.com/goccy/go-json"
	"golang.org/x/exp/slog"

	"github.com/induzo/gocom/contextslogger"
)

type Resp interface {
	// Render will render the response.
	Render(ctx context.Context, respW http.ResponseWriter, encoding Encoding)
}

type ErrResp interface {
	Render(ctx context.Context, respW http.ResponseWriter, encoding Encoding)
	Log(log *slog.Logger)
	IsNil() bool
}

// TypedHandler is the handler that you are actually handling the response.
type TypedHandler[R Resp, ER ErrResp] func(r *http.Request) (R, ER)

// Wrapper will actually do the boring work of logging an error and render the response.
func Wrapper[R Resp, ER ErrResp](f TypedHandler[R, ER]) http.HandlerFunc {
	return http.HandlerFunc(
		func(respW http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			log := contextslogger.FromContext(ctx)
			res, err := f(req)

			encoding := ParseAcceptedEncoding(req)

			if !err.IsNil() {
				err.Log(log)

				err.Render(ctx, respW, encoding)

				return
			}

			res.Render(ctx, respW, encoding)
		},
	)
}

// Render renders a http response, where the content type the response should take is specified by encoding.
// "application/json" is the default content type.
func Render(
	ctx context.Context,
	headers map[string]string,
	statusCode int,
	responseBody interface{},
	respEncoding Encoding,
	respW http.ResponseWriter,
) {
	log := contextslogger.FromContext(ctx)

	//nolint:gocritic,exhaustive // LATER: add more encodings to fix this
	switch respEncoding {
	default:
		for header, headerValue := range headers {
			respW.Header().Add(header, headerValue)
		}

		if responseBody != nil {
			respData, err := json.MarshalContext(ctx, responseBody)
			if err != nil {
				log.Error(
					"http render marshal",
					slog.Any("err", err),
				)

				respW.WriteHeader(http.StatusInternalServerError)

				return
			}

			respW.Header().Add("Content-Type", "application/json")
			respW.WriteHeader(statusCode)

			if _, err := respW.Write(respData); err != nil {
				log.Error(
					"http render write",
					slog.Any("err", err),
				)
			}
		} else {
			respW.WriteHeader(statusCode)
		}
	}
}
