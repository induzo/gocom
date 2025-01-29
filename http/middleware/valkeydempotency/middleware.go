package valkeydempotency

import (
	"fmt"
	"net/http"
	"time"

	"github.com/valkey-io/valkey-go/valkeylock"

	"github.com/induzo/gocom/http/middleware/idempotency"
)

// Middleware enforces idempotency on non-GET requests.
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
