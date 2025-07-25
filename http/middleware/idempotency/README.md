<!-- Code generated by gomarkdoc. DO NOT EDIT -->

# idempotency

```go
import "github.com/induzo/gocom/http/middleware/idempotency"
```

Package idempotency provides an HTTP middleware for managing idempotency. Idempotency ensures that multiple identical requests have the same effect as making a single request, which is useful for operations like payment processing where duplicate requests could lead to unintended consequences. This package is an http middleware that does manage idempotency.

## Index

- [Constants](<#constants>)
- [func ErrorToHTTPJSONProblemDetail\(respW http.ResponseWriter, req \*http.Request, err error\)](<#ErrorToHTTPJSONProblemDetail>)
- [func NewMiddleware\(store Store, options ...Option\) func\(http.Handler\) http.Handler](<#NewMiddleware>)
- [type ErrorToHTTPFn](<#ErrorToHTTPFn>)
- [type GetStoredResponseError](<#GetStoredResponseError>)
  - [func \(e GetStoredResponseError\) Error\(\) string](<#GetStoredResponseError.Error>)
  - [func \(e GetStoredResponseError\) Unwrap\(\) error](<#GetStoredResponseError.Unwrap>)
- [type InMemStore](<#InMemStore>)
  - [func NewInMemStore\(\) \*InMemStore](<#NewInMemStore>)
  - [func \(s \*InMemStore\) GetStoredResponse\(\_ context.Context, key string\) \(\*StoredResponse, bool, error\)](<#InMemStore.GetStoredResponse>)
  - [func \(s \*InMemStore\) StoreResponse\(\_ context.Context, key string, resp \*StoredResponse\) error](<#InMemStore.StoreResponse>)
  - [func \(s \*InMemStore\) TryLock\(ctx context.Context, key string\) \(context.Context, context.CancelFunc, error\)](<#InMemStore.TryLock>)
- [type MismatchedSignatureError](<#MismatchedSignatureError>)
  - [func \(e MismatchedSignatureError\) Error\(\) string](<#MismatchedSignatureError.Error>)
- [type MissingIdempotencyKeyHeaderError](<#MissingIdempotencyKeyHeaderError>)
  - [func \(e MissingIdempotencyKeyHeaderError\) Error\(\) string](<#MissingIdempotencyKeyHeaderError.Error>)
- [type Option](<#Option>)
  - [func WithAffectedMethods\(methods ...string\) Option](<#WithAffectedMethods>)
  - [func WithErrorToHTTPFn\(fn func\(http.ResponseWriter, \*http.Request, error\)\) Option](<#WithErrorToHTTPFn>)
  - [func WithFingerprinter\(fn func\(\*http.Request\) \(\[\]byte, error\)\) Option](<#WithFingerprinter>)
  - [func WithIdempotencyKeyHeader\(header string\) Option](<#WithIdempotencyKeyHeader>)
  - [func WithIdempotentReplayedHeader\(header string\) Option](<#WithIdempotentReplayedHeader>)
  - [func WithIgnoredURLPaths\(urlPaths ...string\) Option](<#WithIgnoredURLPaths>)
  - [func WithOptionalIdempotencyKey\(\) Option](<#WithOptionalIdempotencyKey>)
- [type ProblemDetail](<#ProblemDetail>)
- [type RequestContext](<#RequestContext>)
  - [func \(idrc RequestContext\) String\(\) string](<#RequestContext.String>)
- [type RequestInFlightError](<#RequestInFlightError>)
  - [func \(e RequestInFlightError\) Error\(\) string](<#RequestInFlightError.Error>)
- [type Store](<#Store>)
- [type StoreResponseError](<#StoreResponseError>)
  - [func \(e StoreResponseError\) Error\(\) string](<#StoreResponseError.Error>)
  - [func \(e StoreResponseError\) Unwrap\(\) error](<#StoreResponseError.Unwrap>)
- [type StoredResponse](<#StoredResponse>)


## Constants

<a name="DefaultIdempotencyKeyHeader"></a>

```go
const (
    DefaultIdempotencyKeyHeader             = "X-Idempotency-Key"
    DefaultIdempotentReplayedResponseHeader = "X-Idempotent-Replayed"
)
```

<a name="ErrorToHTTPJSONProblemDetail"></a>
## func [ErrorToHTTPJSONProblemDetail](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/errors.go#L103-L107>)

```go
func ErrorToHTTPJSONProblemDetail(respW http.ResponseWriter, req *http.Request, err error)
```

ErrorToHTTPJSONProblemDetail converts an error to a RFC9457 problem detail. This is a sample errorToHTTPFn function that handles the specific errors encountered You can add your own func and set it inside the config

<a name="NewMiddleware"></a>
## func [NewMiddleware](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/middleware.go#L13>)

```go
func NewMiddleware(store Store, options ...Option) func(http.Handler) http.Handler
```

Middleware enforces idempotency on non\-GET requests.

<details><summary>Example</summary>
<p>

Using NewMiddleware

```go
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/induzo/gocom/http/middleware/idempotency"
)

// Using NewMiddleware
func main() {
	ctx := context.Background()
	idempotencyMiddleware := idempotency.NewMiddleware(idempotency.NewInMemStore())
	mux := http.NewServeMux()

	counter := int32(0)

	mux.Handle("/",
		idempotencyMiddleware(
			http.HandlerFunc(func(respW http.ResponseWriter, _ *http.Request) {
				time.Sleep(100 * time.Millisecond)

				atomic.AddInt32(&counter, 1)

				respW.Write([]byte("Hello World! " + strconv.Itoa(int(counter))))
			})),
	)

	// Serve the handler with http test server
	server := httptest.NewServer(mux)
	defer server.Close()

	// send a first req without a key
	sendPOSTReq(ctx, server, "", "")

	var wg sync.WaitGroup

	for range 3 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			sendPOSTReq(ctx, server, "same-key", "")
		}()

		time.Sleep(80 * time.Millisecond)
	}

}

func sendPOSTReq(ctx context.Context, server *httptest.Server, key, reqBody string) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, bytes.NewBufferString(reqBody))
	req.Header.Set(idempotency.DefaultIdempotencyKeyHeader, key)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(err)

		return
	}
	defer resp.Body.Close()

	fmt.Println(resp.StatusCode)

	body, errB := io.ReadAll(resp.Body)
	if errB != nil {
		fmt.Println(errB)

		return
	}

	fmt.Println(string(body))
}
```

#### Output

```
400
{
  "type": "errors/missing-idempotency-key-header",
  "title": "missing idempotency key header",
  "detail": "missing idempotency key header `X-Idempotency-Key`",
  "instance": "/"
}
409
{
  "type": "errors/request-already-in-flight",
  "title": "request already in flight",
  "detail": "request with key `X-Idempotency-Key`:`same-key` still in flight",
  "instance": "/"
}
200
Hello World! 1
200
Hello World! 1
```

</p>
</details>

<a name="ErrorToHTTPFn"></a>
## type [ErrorToHTTPFn](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/config.go#L12>)



```go
type ErrorToHTTPFn func(http.ResponseWriter, *http.Request, error)
```

<a name="GetStoredResponseError"></a>
## type [GetStoredResponseError](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/errors.go#L72-L75>)



```go
type GetStoredResponseError struct {
    RequestContext
    Err error
}
```

<a name="GetStoredResponseError.Error"></a>
### func \(GetStoredResponseError\) [Error](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/errors.go#L77>)

```go
func (e GetStoredResponseError) Error() string
```



<a name="GetStoredResponseError.Unwrap"></a>
### func \(GetStoredResponseError\) [Unwrap](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/errors.go#L85>)

```go
func (e GetStoredResponseError) Unwrap() error
```



<a name="InMemStore"></a>
## type [InMemStore](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/inmem.go#L11-L16>)



```go
type InMemStore struct {
    // contains filtered or unexported fields
}
```

<a name="NewInMemStore"></a>
### func [NewInMemStore](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/inmem.go#L19>)

```go
func NewInMemStore() *InMemStore
```

NewInMemStore initializes an in\-memory store.

<a name="InMemStore.GetStoredResponse"></a>
### func \(\*InMemStore\) [GetStoredResponse](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/inmem.go#L56>)

```go
func (s *InMemStore) GetStoredResponse(_ context.Context, key string) (*StoredResponse, bool, error)
```



<a name="InMemStore.StoreResponse"></a>
### func \(\*InMemStore\) [StoreResponse](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/inmem.go#L44>)

```go
func (s *InMemStore) StoreResponse(_ context.Context, key string, resp *StoredResponse) error
```



<a name="InMemStore.TryLock"></a>
### func \(\*InMemStore\) [TryLock](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/inmem.go#L34>)

```go
func (s *InMemStore) TryLock(ctx context.Context, key string) (context.Context, context.CancelFunc, error)
```



<a name="MismatchedSignatureError"></a>
## type [MismatchedSignatureError](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/errors.go#L47-L49>)



```go
type MismatchedSignatureError struct {
    RequestContext
}
```

<a name="MismatchedSignatureError.Error"></a>
### func \(MismatchedSignatureError\) [Error](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/errors.go#L51>)

```go
func (e MismatchedSignatureError) Error() string
```



<a name="MissingIdempotencyKeyHeaderError"></a>
## type [MissingIdempotencyKeyHeaderError](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/errors.go#L31-L33>)



```go
type MissingIdempotencyKeyHeaderError struct {
    RequestContext
}
```

<a name="MissingIdempotencyKeyHeaderError.Error"></a>
### func \(MissingIdempotencyKeyHeaderError\) [Error](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/errors.go#L35>)

```go
func (e MissingIdempotencyKeyHeaderError) Error() string
```



<a name="Option"></a>
## type [Option](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/options.go#L7>)



```go
type Option func(*config)
```

<a name="WithAffectedMethods"></a>
### func [WithAffectedMethods](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/options.go#L46>)

```go
func WithAffectedMethods(methods ...string) Option
```

WithAffectedMethods sets the methods that are affected by idempotency. By default, POST only are affected.

<a name="WithErrorToHTTPFn"></a>
### func [WithErrorToHTTPFn](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/options.go#L31>)

```go
func WithErrorToHTTPFn(fn func(http.ResponseWriter, *http.Request, error)) Option
```

WithErrorToHTTP sets a function to convert errors to HTTP status codes and content.

<a name="WithFingerprinter"></a>
### func [WithFingerprinter](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/options.go#L38>)

```go
func WithFingerprinter(fn func(*http.Request) ([]byte, error)) Option
```

WithFingerprinter sets a function to build a request fingerprint.

<a name="WithIdempotencyKeyHeader"></a>
### func [WithIdempotencyKeyHeader](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/options.go#L17>)

```go
func WithIdempotencyKeyHeader(header string) Option
```

WithIdempotencyKeyHeader sets the header to use for idempotency keys.

<a name="WithIdempotentReplayedHeader"></a>
### func [WithIdempotentReplayedHeader](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/options.go#L24>)

```go
func WithIdempotentReplayedHeader(header string) Option
```

WithIdempotentReplayedHeader sets the header to use for idempotent replayed responses.

<a name="WithIgnoredURLPaths"></a>
### func [WithIgnoredURLPaths](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/options.go#L54>)

```go
func WithIgnoredURLPaths(urlPaths ...string) Option
```

WithIgnoredURLPaths sets the URL paths that are ignored by idempotency. By default, no URLs are ignored.

<a name="WithOptionalIdempotencyKey"></a>
### func [WithOptionalIdempotencyKey](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/options.go#L10>)

```go
func WithOptionalIdempotencyKey() Option
```

WithOptionalIdempotencyKey sets the idempotency key to optional.

<a name="ProblemDetail"></a>
## type [ProblemDetail](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/errors.go#L90-L98>)

Conforming to RFC9457 \(https://www.rfc-editor.org/rfc/rfc9457.html\)

```go
type ProblemDetail struct {
    HTTPStatusCode int `json:"-"`

    Type             string         `json:"type"`
    Title            string         `json:"title"`
    Detail           string         `json:"detail"`
    Instance         string         `json:"instance"`
    ExtensionMembers map[string]any `json:",omitempty"`
}
```

<a name="RequestContext"></a>
## type [RequestContext](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/errors.go#L11-L16>)



```go
type RequestContext struct {
    URL       string
    Method    string
    KeyHeader string
    Key       string
}
```

<a name="RequestContext.String"></a>
### func \(RequestContext\) [String](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/errors.go#L18>)

```go
func (idrc RequestContext) String() string
```



<a name="RequestInFlightError"></a>
## type [RequestInFlightError](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/errors.go#L39-L41>)



```go
type RequestInFlightError struct {
    RequestContext
}
```

<a name="RequestInFlightError.Error"></a>
### func \(RequestInFlightError\) [Error](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/errors.go#L43>)

```go
func (e RequestInFlightError) Error() string
```



<a name="Store"></a>
## type [Store](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/store.go#L21-L32>)

Store is the interface we need to implement for: Locking an idemkey Storing a response Retrieving a response

```go
type Store interface {
    // Lock inserts a marker that a request with a given key/signature is in-flight.
    TryLock(ctx context.Context, key string) (context.Context, context.CancelFunc, error)

    // MarkComplete records the final response for a request key.
    StoreResponse(ctx context.Context, key string, resp *StoredResponse) error

    // GetStoredResponse returns the final stored response (if any) for this key.
    // The second return value is false if the key is not found.
    // The third return value is an error if the operation failed.
    GetStoredResponse(ctx context.Context, key string) (*StoredResponse, bool, error)
}
```

<a name="StoreResponseError"></a>
## type [StoreResponseError](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/errors.go#L55-L58>)



```go
type StoreResponseError struct {
    RequestContext
    Err error
}
```

<a name="StoreResponseError.Error"></a>
### func \(StoreResponseError\) [Error](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/errors.go#L60>)

```go
func (e StoreResponseError) Error() string
```



<a name="StoreResponseError.Unwrap"></a>
### func \(StoreResponseError\) [Unwrap](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/errors.go#L68>)

```go
func (e StoreResponseError) Unwrap() error
```



<a name="StoredResponse"></a>
## type [StoredResponse](<https://github.com/induzo/gocom/blob/main/http/middleware/idempotency/store.go#L9-L15>)

StoredResponse holds what we need to check and replay a response.

```go
type StoredResponse struct {
    StatusCode  int
    Signature   []byte
    Header      http.Header
    Body        []byte
    RequestHash []byte // To verify the same request payload
}
```

Generated by [gomarkdoc](<https://github.com/princjef/gomarkdoc>)
