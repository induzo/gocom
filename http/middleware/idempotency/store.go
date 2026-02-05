package idempotency

import (
	"context"
	"net/http"
	"time"
)

// StoredResponse holds what we need to check and replay a response.
type StoredResponse struct {
	StatusCode  int
	Signature   []byte
	Header      http.Header
	Body        []byte
	RequestHash []byte // To verify the same request payload
}

// Store is the interface we need to implement for:
// Locking an idemkey
// Storing a response
// Retrieving a response
type Store interface {
	// Lock inserts a marker that a request with a given key/signature is in-flight.
	// The lock should have a timeout to prevent indefinite holding.
	TryLock(ctx context.Context, key string) (context.Context, context.CancelFunc, error)

	// MarkComplete records the final response for a request key.
	// The TTL is configured via SetResponseTTL.
	StoreResponse(ctx context.Context, key string, resp *StoredResponse) error

	// GetStoredResponse returns the final stored response (if any) for this key.
	// The second return value is false if the key is not found.
	// The third return value is an error if the operation failed.
	GetStoredResponse(ctx context.Context, key string) (*StoredResponse, bool, error)

	// SetResponseTTL configures how long responses should be cached.
	// This is called by the middleware during initialization.
	SetResponseTTL(ttl time.Duration)
}
