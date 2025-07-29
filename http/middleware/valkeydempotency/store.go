package valkeydempotency

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/valkey-io/valkey-go"
	"github.com/valkey-io/valkey-go/valkeylock"

	"github.com/induzo/gocom/http/middleware/idempotency"
)

// it must implement the store interface
// type Store interface {
// 	TryLock(ctx context.Context, key string) (context.Context, context.CancelFunc, error)
// 	StoreResponse(ctx context.Context, key string, resp *StoredResponse) error
// 	GetStoredResponse(ctx context.Context, key string) (*StoredResponse, bool, error)
// }

var _ idempotency.Store = &Store{}

type Store struct {
	locker                   valkeylock.Locker
	client                   valkey.Client
	storedResponseTTLSeconds int64 // time to live, can not be below 1 second
}

func (sto *Store) Close() error {
	sto.locker.Close()

	return nil
}

type TTLIncorrectError struct {
	ttl time.Duration
}

func (e *TTLIncorrectError) Error() string {
	return fmt.Sprintf("ttl must be greater than 0, got %s", e.ttl)
}

func NewStore(lockerOption *valkeylock.LockerOption, ttl time.Duration) (*Store, error) {
	locker, err := valkeylock.NewLocker(*lockerOption)
	if err != nil {
		return nil, fmt.Errorf("failed to create valkey locker: %w", err)
	}

	if ttl <= 0 {
		return nil, &TTLIncorrectError{
			ttl: ttl,
		}
	}

	return &Store{
			locker:                   locker,
			client:                   locker.Client(),
			storedResponseTTLSeconds: int64(ttl.Seconds()),
		},
		nil
}

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

	if errD := sto.client.Do(
		ctx,
		sto.client.B().
			Setex().
			Key(key).
			Seconds(sto.storedResponseTTLSeconds).
			Value(string(serializedResp)).
			Build(),
	).Error(); errD != nil {
		return fmt.Errorf("failed to store response: %w", errD)
	}

	return nil
}

func (sto *Store) GetStoredResponse(
	ctx context.Context,
	key string,
) (*idempotency.StoredResponse, bool, error) {
	var storedResp idempotency.StoredResponse

	respB, err := sto.client.Do(
		ctx,
		sto.client.B().
			Get().
			Key(key).
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
