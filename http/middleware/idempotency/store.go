package idempotency

import (
	"context"
	"net/http"
)

// StoredResponse holds what we need to check and replay a response.
//
// Signature is reserved for future use; the canonical hash field is
// RequestHash. Implementations of [Store] should leave Signature unset.
type StoredResponse struct {
	StatusCode  int
	Signature   []byte // reserved; not currently used by the middleware
	Header      http.Header
	Body        []byte
	RequestHash []byte // hash of the canonical request used to detect mismatched payloads
}

// Store is the persistence interface implementations must satisfy. The
// middleware uses it to (1) take an in-flight lock on a composite key, (2)
// persist the final response so subsequent identical requests can replay,
// and (3) retrieve a previously stored response.
//
// Implementations are expected to be safe for concurrent use.
type Store interface {
	// TryLock attempts to acquire an in-flight lock for key. It returns a
	// context that scopes the lock's lifetime, a cancel function the caller
	// must invoke once the request has completed, and a non-nil error if
	// the lock is already held by another in-flight request. The lock
	// should have a server-side timeout so a crashed handler does not hold
	// it forever.
	TryLock(ctx context.Context, key string) (context.Context, context.CancelFunc, error)

	// StoreResponse records the final response for key. Implementations
	// must overwrite any previously stored value for the same key (last
	// writer wins) so that retries after a transient failure can succeed.
	StoreResponse(ctx context.Context, key string, resp *StoredResponse) error

	// GetStoredResponse returns the stored response for key, if any. The
	// second return value is false when the key is not (or no longer)
	// present; the third is non-nil if the lookup itself failed.
	GetStoredResponse(ctx context.Context, key string) (*StoredResponse, bool, error)
}
