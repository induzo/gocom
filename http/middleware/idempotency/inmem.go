package idempotency

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var _ Store = &InMemStore{}

// InMemStore is a single-process [Store] backed by [sync.Map]. It is
// suitable for tests and single-instance deployments; production
// deployments running multiple replicas should use a shared backend such
// as the sibling valkeydempotency package.
type InMemStore struct {
	locks                      sync.Map // key -> time.Time (lock expiry)
	responses                  sync.Map // key -> *storedEntry
	withStoreResponseError     bool
	withGetStoredResponseError bool
	lockTimeout                time.Duration
	responseTTL                time.Duration
	cancel                     context.CancelFunc
	closeOnce                  sync.Once
}

type storedEntry struct {
	response  *StoredResponse
	expiresAt time.Time
}

const (
	defaultLockTimeout = 30 * time.Second
	defaultResponseTTL = 24 * time.Hour
)

// NewInMemStore initializes an in-memory store with automatic cleanup.
func NewInMemStore() *InMemStore {
	ctx, cancel := context.WithCancel(context.Background())

	store := &InMemStore{
		locks:       sync.Map{},
		responses:   sync.Map{},
		lockTimeout: defaultLockTimeout,
		responseTTL: defaultResponseTTL,
		cancel:      cancel,
	}

	// Start background cleanup goroutine.
	go store.cleanup(ctx)

	return store
}

// Close stops the background cleanup goroutine. Safe to call more than
// once.
func (s *InMemStore) Close() {
	s.closeOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
	})
}

// cleanup periodically removes expired locks and responses.
func (s *InMemStore) cleanup(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()

			// Clean expired locks.
			s.locks.Range(func(key, value any) bool {
				if expiry, ok := value.(time.Time); ok && now.After(expiry) {
					s.locks.Delete(key)
				}

				return true
			})

			// Clean expired responses.
			s.responses.Range(func(key, value any) bool {
				if entry, ok := value.(*storedEntry); ok && now.After(entry.expiresAt) {
					s.responses.Delete(key)
				}

				return true
			})
		}
	}
}

// TryLock attempts to take an in-flight lock for key. The fast path uses
// LoadOrStore. If the existing lock has expired, TryLock retries via
// CompareAndSwap so that two concurrent callers cannot both observe an
// expired lock and both believe they hold it.
func (s *InMemStore) TryLock(
	ctx context.Context,
	key string,
) (context.Context, context.CancelFunc, error) {
	for {
		now := time.Now()
		lockExpiry := now.Add(s.lockTimeout)

		existing, loaded := s.locks.LoadOrStore(key, lockExpiry)
		if !loaded {
			// We won the race; fresh lock acquired.
			return ctx, func() { s.locks.Delete(key) }, nil
		}

		expiry, ok := existing.(time.Time)
		if !ok {
			return ctx, nil, fmt.Errorf(
				"TryLock: %w",
				errors.New("unexpected lock value type"), //nolint:err113 // local fixture
			)
		}

		if now.Before(expiry) {
			return ctx, nil, fmt.Errorf(
				"TryLock: %w",
				errors.New("key is already locked"), //nolint:err113 // local fixture
			)
		}

		// Lock expired. Atomically replace it; if CAS fails another
		// caller swooped in, so retry.
		if s.locks.CompareAndSwap(key, existing, lockExpiry) {
			return ctx, func() { s.locks.Delete(key) }, nil
		}
	}
}

// StoreResponse persists resp under key with the configured response TTL.
// Subsequent calls overwrite any previous entry so retries after a
// transient failure can succeed.
func (s *InMemStore) StoreResponse(
	_ context.Context,
	key string,
	resp *StoredResponse,
) error {
	if s.withStoreResponseError {
		return fmt.Errorf(
			"StoreResponse: %w",
			errors.New("store error"), //nolint:err113 // local fixture
		)
	}

	entry := &storedEntry{
		response:  resp,
		expiresAt: time.Now().Add(s.responseTTL),
	}

	s.responses.Store(key, entry)

	return nil
}

// GetStoredResponse returns the response previously stored for key, or
// (nil, false, nil) if none exists or the entry has expired.
func (s *InMemStore) GetStoredResponse(
	_ context.Context,
	key string,
) (*StoredResponse, bool, error) {
	if s.withGetStoredResponseError {
		return nil,
			false,
			fmt.Errorf(
				"GetStoredResponse: %w",
				errors.New("get stored response error"), //nolint:err113 // local fixture
			)
	}

	value, found := s.responses.Load(key)
	if !found || value == nil {
		return nil, false, nil
	}

	entry, valid := value.(*storedEntry)
	if !valid {
		return nil,
			false,
			fmt.Errorf(
				"GetStoredResponse: %w",
				errors.New("unexpected response type"), //nolint:err113 // local fixture
			)
	}

	// Check if expired.
	if time.Now().After(entry.expiresAt) {
		s.responses.Delete(key)

		return nil, false, nil
	}

	return entry.response, true, nil
}

// SetResponseTTL configures how long responses should be cached.
func (s *InMemStore) SetResponseTTL(ttl time.Duration) {
	s.responseTTL = ttl
}
