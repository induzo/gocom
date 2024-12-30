package idempotency

import (
	"context"
	"net/http"
)

// StoredResponse holds what we need to “replay” a response.
type StoredResponse struct {
	StatusCode       int
	Headers          http.Header
	Body             []byte
	RequestSignature []byte // To verify the same request payload
}

// Store is the interface we need to implement for storing:
// - 'in-flight' markers
// - final responses
type Store interface {
	// InsertInFlight inserts a marker that a request with a given key/signature is in-flight.
	InsertInFlight(ctx context.Context, key string, requestSignature []byte) error

	// MarkComplete records the final response for a request key.
	MarkComplete(ctx context.Context, key string, resp *StoredResponse) error

	// GetInFlightSignature returns the signature for an in-flight request (if any).
	GetInFlightSignature(ctx context.Context, key string) ([]byte, bool, error)

	// GetStoredResponse returns the final stored response (if any) for this key.
	GetStoredResponse(ctx context.Context, key string) (*StoredResponse, bool, error)

	// RemoveInFlight removes the in-flight marker without storing a response (e.g., if there's an error).
	RemoveInFlight(ctx context.Context, key string) error
}
