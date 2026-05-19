package valkeydempotency

import (
	"fmt"
	"net/http"
	"time"

	"github.com/valkey-io/valkey-go/valkeylock"

	"github.com/triple-a/gocom/http/middleware/idempotency"
)

// NewMiddleware constructs a Valkey-backed [idempotency.Store] and wraps
// it with [idempotency.NewMiddleware]. It returns the middleware, a
// closer that releases the underlying valkeylock client, and any setup
// error.
//
// ttl is the response cache TTL; it must be at least one second
// (sub-second TTLs return TTLIncorrectError because Valkey SETEX rejects
// 0-second expirations at runtime). lockerOption must be non-nil.
func NewMiddleware(
	lockerOption *valkeylock.LockerOption,
	ttl time.Duration,
	options ...idempotency.Option,
) (func(http.Handler) http.Handler, func() error, error) {
	store, errS := NewStore(lockerOption, ttl)
	if errS != nil {
		return nil, nil, fmt.Errorf("failed to create valkey idempotency store: %w", errS)
	}

	return idempotency.NewMiddleware(store, options...), store.Close, nil
}
