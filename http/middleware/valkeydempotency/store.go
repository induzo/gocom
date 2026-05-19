package valkeydempotency

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/valkey-io/valkey-go"
	"github.com/valkey-io/valkey-go/valkeylock"

	"github.com/triple-a/gocom/http/middleware/idempotency"
)

// responseKeyPrefix scopes stored-response keys so they cannot collide
// with the locker's own keyspace (whose prefix is configured via
// valkeylock.LockerOption.KeyPrefix, default "rwlock") nor with arbitrary
// keys written by other tenants of the same Valkey deployment.
const responseKeyPrefix = "idemresp:"

// defaultStoreWriteTimeout bounds the write made by StoreResponse when
// the request context has been detached. 10 seconds is generous compared
// to typical Valkey latency but short enough that a hung connection does
// not pin a goroutine indefinitely.
const defaultStoreWriteTimeout = 10 * time.Second

// minStoreTTL is the smallest response TTL the store accepts. Sub-second
// TTLs cause Valkey SETEX to error at runtime because the seconds-truncated
// value is zero, so we reject them up front.
const minStoreTTL = time.Second

// ErrNilLockerOption is returned by NewStore when the lockerOption
// argument is nil.
var ErrNilLockerOption = errors.New("valkeylock.LockerOption must not be nil")

var _ idempotency.Store = &Store{}

// Store implements idempotency.Store backed by Valkey: distributed
// in-flight locks via valkeylock and SETEX-cached responses on the same
// Valkey client.
type Store struct {
	locker                   valkeylock.Locker
	client                   valkey.Client
	storedResponseTTLSeconds int64 // bound by minStoreTTL
}

// Close releases the underlying valkeylock and its Valkey client. The
// upstream Locker.Close has no error return, so Close is signature-only
// and always returns nil; it is safe to call more than once because
// valkeylock guards against double-close internally.
func (sto *Store) Close() error {
	sto.locker.Close()

	return nil
}

// TTLIncorrectError is returned by NewStore when the supplied TTL is
// shorter than minStoreTTL.
type TTLIncorrectError struct {
	ttl time.Duration
}

// Error implements error.
func (e *TTLIncorrectError) Error() string {
	return fmt.Sprintf("ttl must be at least %s, got %s", minStoreTTL, e.ttl)
}

// NewStore initializes a Valkey-backed Store. lockerOption must be
// non-nil; ttl must be at least one second.
func NewStore(lockerOption *valkeylock.LockerOption, ttl time.Duration) (*Store, error) {
	if lockerOption == nil {
		return nil, ErrNilLockerOption //nolint:wrapcheck // sentinel returned as-is for errors.Is.
	}

	if ttl < minStoreTTL {
		return nil, &TTLIncorrectError{ttl: ttl}
	}

	locker, err := valkeylock.NewLocker(*lockerOption)
	if err != nil {
		return nil, fmt.Errorf("failed to create valkey locker: %w", err)
	}

	return &Store{
		locker:                   locker,
		client:                   locker.Client(),
		storedResponseTTLSeconds: int64(ttl.Seconds()),
	}, nil
}

// TryLock takes a Valkey-backed Redlock-style lock for key. The
// underlying valkeylock library handles auto-extension under the
// configured majority.
func (sto *Store) TryLock(
	ctx context.Context,
	key string,
) (context.Context, context.CancelFunc, error) {
	ctx, cancel, err := sto.locker.TryWithContext(ctx, key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to lock key %s: %w", key, err)
	}

	return ctx, cancel, nil
}

// StoreResponse persists the response under the namespaced key with the
// configured TTL. It detaches from ctx via context.WithoutCancel so a
// client disconnect between handler completion and the SETEX call does
// not leave the cache empty (which would cause the next identical
// request to re-execute the handler). The detached call is bounded by
// defaultStoreWriteTimeout.
func (sto *Store) StoreResponse(
	ctx context.Context,
	key string,
	resp *idempotency.StoredResponse,
) error {
	// serialize the response
	serializedResp, errM := json.Marshal(resp)
	if errM != nil {
		return fmt.Errorf("failed to serialize response: %w", errM)
	}

	writeCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		defaultStoreWriteTimeout,
	)
	defer cancel()

	if errD := sto.client.Do(
		writeCtx,
		sto.client.B().
			Setex().
			Key(responseKeyPrefix+key).
			Seconds(sto.storedResponseTTLSeconds).
			Value(string(serializedResp)).
			Build(),
	).Error(); errD != nil {
		return fmt.Errorf("failed to store response: %w", errD)
	}

	return nil
}

// GetStoredResponse fetches a previously stored response for key, or
// (nil, false, nil) if no entry exists or it has expired.
func (sto *Store) GetStoredResponse(
	ctx context.Context,
	key string,
) (*idempotency.StoredResponse, bool, error) {
	var storedResp idempotency.StoredResponse

	respB, err := sto.client.Do(
		ctx,
		sto.client.B().
			Get().
			Key(responseKeyPrefix+key).
			Build(),
	).AsBytes()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return nil, false, nil
		}

		return nil, false, fmt.Errorf("failed to get response: %w", err)
	}

	if errU := json.Unmarshal(respB, &storedResp); errU != nil {
		return nil, false, fmt.Errorf("failed to unmarshal response: %w", errU)
	}

	return &storedResp, true, nil
}
