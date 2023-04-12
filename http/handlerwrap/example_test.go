package handlerwrap_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"

	"golang.org/x/exp/slog"

	"github.com/induzo/gocom/contextslogger"
	"github.com/induzo/gocom/http/handlerwrap"
)

// Wrapping a POST http handler.
func ExampleTypedHandler_post() {
	type postRequest struct {
		Name string `json:"name"`
	}

	createHandler := func() handlerwrap.TypedHandler[*handlerwrap.Response, *handlerwrap.ErrorResponse] {
		return func(r *http.Request) (*handlerwrap.Response, *handlerwrap.ErrorResponse) {
			var pr postRequest

			if err := handlerwrap.BindBody(r, &pr); err != nil {
				return nil, err
			}

			log.Println(pr)

			return &handlerwrap.Response{
				Body:       pr,
				Headers:    make(map[string]string),
				StatusCode: http.StatusCreated,
			}, nil
		}
	}

	reqBody, err := json.Marshal(postRequest{
		Name: "test",
	})
	if err != nil {
		log.Fatalf("marshal reqbody: %s", err)
	}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", bytes.NewBuffer(reqBody))
	req = req.WithContext(contextslogger.NewContext(req.Context(), slog.New(slog.NewTextHandler(io.Discard))))

	nr := httptest.NewRecorder()

	handlerwrap.Wrapper(createHandler()).ServeHTTP(nr, req)

	rr := nr.Result()
	defer rr.Body.Close()

	fmt.Println(rr.StatusCode)
	// Output:
	// 201
}

// Wrapping a GET http handler.
func ExampleTypedHandler_get() {
	getter := func(ctx context.Context, key string) (string, *handlerwrap.ErrorResponse) {
		if key == "id" {
			return "1", nil
		}

		missingParamErr := &handlerwrap.MissingParamError{Name: key}

		return "", missingParamErr.ToErrorResponse()
	}

	getHandler := func(nupg handlerwrap.NamedURLParamsGetter) handlerwrap.TypedHandler[*handlerwrap.Response, *handlerwrap.ErrorResponse] {
		return func(r *http.Request) (*handlerwrap.Response, *handlerwrap.ErrorResponse) {
			idParam, errR := nupg(r.Context(), "id")
			if errR != nil {
				return nil, errR
			}

			log.Println(idParam)

			id, err := strconv.ParseInt(idParam, 10, 64)
			if err != nil {
				parsingParamErr := &handlerwrap.ParsingParamError{
					Name:  "id",
					Value: idParam,
				}

				return nil, parsingParamErr.ToErrorResponse()
			}

			return &handlerwrap.Response{
				Body:       id,
				Headers:    make(map[string]string),
				StatusCode: http.StatusOK,
			}, nil
		}
	}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req = req.WithContext(contextslogger.NewContext(req.Context(), slog.New(slog.NewTextHandler(io.Discard))))
	nr := httptest.NewRecorder()

	handlerwrap.Wrapper(getHandler(getter)).ServeHTTP(nr, req)

	rr := nr.Result()
	defer rr.Body.Close()

	fmt.Println(rr.StatusCode)
	// Output:
	// 200
}

// Render.
func ExampleRender() {
	handler := func() http.HandlerFunc {
		return http.HandlerFunc(func(respW http.ResponseWriter, req *http.Request) {
			body := struct {
				Test int `json:"test"`
			}{Test: 123}
			headers := map[string]string{}
			statusCode := http.StatusOK

			handlerwrap.Render(req.Context(), headers, statusCode, body, handlerwrap.ApplicationJSON, respW)
		})
	}

	mux := http.NewServeMux()
	mux.Handle("/", handler())

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req = req.WithContext(contextslogger.NewContext(req.Context(), slog.New(slog.NewTextHandler(io.Discard))))
	nr := httptest.NewRecorder()

	mux.ServeHTTP(nr, req)

	rr := nr.Result()
	defer rr.Body.Close()

	body, _ := io.ReadAll(rr.Body)

	fmt.Println(string(body))
	// Output:
	// {"test":123}
}

// Use ParseAcceptedEncoding to get the encoding and use it to render the http response.
func ExampleParseAcceptedEncoding() {
	handler := func() http.HandlerFunc {
		return http.HandlerFunc(func(respW http.ResponseWriter, req *http.Request) {
			body := struct {
				Test int `json:"test"`
			}{Test: 123}
			headers := map[string]string{}
			statusCode := http.StatusOK

			encoding := handlerwrap.ParseAcceptedEncoding(req)

			handlerwrap.Render(req.Context(), headers, statusCode, body, encoding, respW)
		})
	}

	mux := http.NewServeMux()
	mux.Handle("/", handler())

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req = req.WithContext(contextslogger.NewContext(req.Context(), slog.New(slog.NewTextHandler(io.Discard))))
	nr := httptest.NewRecorder()

	mux.ServeHTTP(nr, req)

	rr := nr.Result()
	defer rr.Body.Close()

	body, _ := io.ReadAll(rr.Body)

	fmt.Println(string(body))
	// Output:
	// {"test":123}
}

// Render response.
func ExampleResponse_Render() {
	handler := func() http.HandlerFunc {
		return http.HandlerFunc(func(respW http.ResponseWriter, req *http.Request) {
			resp := &handlerwrap.Response{
				Body: map[string]any{
					"hello": "world",
				},
				Headers:    make(map[string]string),
				StatusCode: http.StatusOK,
			}

			resp.Render(req.Context(), respW, handlerwrap.ApplicationJSON)
		})
	}

	mux := http.NewServeMux()
	mux.Handle("/", handler())

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req = req.WithContext(contextslogger.NewContext(req.Context(), slog.New(slog.NewTextHandler(io.Discard))))
	nr := httptest.NewRecorder()

	mux.ServeHTTP(nr, req)

	rr := nr.Result()
	defer rr.Body.Close()

	body, _ := io.ReadAll(rr.Body)

	fmt.Println(string(body))
	// Output:
	// {"hello":"world"}
}

// Render error response.
func ExampleErrorResponse_Render() {
	handler := func() http.HandlerFunc {
		return http.HandlerFunc(func(respW http.ResponseWriter, req *http.Request) {
			errResp := handlerwrap.NewErrorResponse(
				fmt.Errorf("dummy err"),
				map[string]string{},
				http.StatusInternalServerError,
				"dummy_err",
				"dummy err",
			)

			errResp.Render(req.Context(), respW, handlerwrap.ApplicationJSON)
		})
	}

	mux := http.NewServeMux()
	mux.Handle("/", handler())

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req = req.WithContext(contextslogger.NewContext(req.Context(), slog.New(slog.NewTextHandler(io.Discard))))
	nr := httptest.NewRecorder()

	mux.ServeHTTP(nr, req)

	rr := nr.Result()
	defer rr.Body.Close()

	body, _ := io.ReadAll(rr.Body)

	fmt.Println(string(body))
	// Output:
	// {"error":"dummy_err","error_message":"dummy err"}
}

// Get query parameters using ParsePaginationQueryParams in handler
func ExampleParsePaginationQueryParams() {
	listHandler := func() handlerwrap.TypedHandler[*handlerwrap.Response, *handlerwrap.ErrorResponse] {
		return func(r *http.Request) (*handlerwrap.Response, *handlerwrap.ErrorResponse) {
			paginationParams, err := handlerwrap.ParsePaginationQueryParams(r.URL, "id", 10, 100)
			if err != nil {
				return nil, err
			}

			return &handlerwrap.Response{
				Body:       paginationParams.Limit,
				Headers:    make(map[string]string),
				StatusCode: http.StatusOK,
			}, nil
		}
	}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/limit?=10", nil)
	req = req.WithContext(contextslogger.NewContext(req.Context(), slog.New(slog.NewTextHandler(io.Discard))))
	nr := httptest.NewRecorder()

	handlerwrap.Wrapper(listHandler()).ServeHTTP(nr, req)

	rr := nr.Result()
	defer rr.Body.Close()

	body, _ := io.ReadAll(rr.Body)

	fmt.Println(string(body))
	// Output:
	// 10
}
