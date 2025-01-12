package idempotency

import (
	"context"
	"net/http"
)

// StoredResponse holds what we need to check and replay a response.
type StoredResponse struct {
	StatusCode       int
	Signature        []byte
	Headers          http.Header
	Body             []byte
	RequestSignature []byte // To verify the same request payload
}

// Store is the interface we need to implement for:
// Locking an idemkey
// Storing a response
// Retrieving a response
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
