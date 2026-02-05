package idempotency

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var _ Store = &InMemStore{}

type storedEntry struct {
	response  *StoredResponse
	expiresAt time.Time
}

type InMemStore struct {
	locks                      sync.Map // key -> time.Time (lock expiry)
	responses                  sync.Map // key -> *storedEntry
	withStoreResponseError     bool
	withGetStoredResponseError bool
	lockTimeout                time.Duration
	responseTTL                time.Duration
	cancel                     context.CancelFunc
}

const defaultLockTimeout = 30 * time.Second

// NewInMemStore initializes an in-memory store with automatic cleanup.
func NewInMemStore() *InMemStore {
	ctx, cancel := context.WithCancel(context.Background())

	store := &InMemStore{
		locks:       sync.Map{},
		responses:   sync.Map{},
		lockTimeout: defaultLockTimeout,
		responseTTL: DefaultResponseTTL,
		cancel:      cancel,
	}

	// Start background cleanup goroutine
	go store.cleanup(ctx)

	return store
}

// Close stops the background cleanup goroutine.
func (s *InMemStore) Close() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *InMemStore) withActiveStoreResponseError() {
	s.withStoreResponseError = true
}

func (s *InMemStore) withActiveGetStoredResponseError() {
	s.withGetStoredResponseError = true
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

			// Clean expired locks
			s.locks.Range(func(key, value any) bool {
				if expiry, ok := value.(time.Time); ok && now.After(expiry) {
					s.locks.Delete(key)
				}

				return true
			})

			// Clean expired responses
			s.responses.Range(func(key, value any) bool {
				if entry, ok := value.(*storedEntry); ok && now.After(entry.expiresAt) {
					s.responses.Delete(key)
				}

				return true
			})
		}
	}
}

func (s *InMemStore) TryLock(
	ctx context.Context,
	key string,
) (context.Context, context.CancelFunc, error) {
	now := time.Now()
	lockExpiry := now.Add(s.lockTimeout)

	// Check if lock exists and is not expired
	if existing, loaded := s.locks.LoadOrStore(key, lockExpiry); loaded {
		if expiry, ok := existing.(time.Time); ok && now.Before(expiry) {
			return ctx,
				nil,
				fmt.Errorf(
					"TryLock: %w",
					errors.New("key is already locked"), //nolint:err113 // this is a test store
				)
		}

		// Lock expired, update it
		s.locks.Store(key, lockExpiry)
	}

	return ctx, func() {
		s.locks.Delete(key)
	}, nil
}

func (s *InMemStore) StoreResponse(
	_ context.Context,
	key string,
	resp *StoredResponse,
) error {
	if s.withStoreResponseError {
		return fmt.Errorf(
			"StoreResponse: %w",
			errors.New("store error"), //nolint:err113 // this is a test store
		)
	}

	entry := &storedEntry{
		response:  resp,
		expiresAt: time.Now().Add(s.responseTTL),
	}

	if _, loaded := s.responses.LoadOrStore(key, entry); loaded {
		return fmt.Errorf(
			"StoreResponse: %w",
			errors.New("key already present"), //nolint:err113 // this is a test store
		)
	}

	return nil
}

func (s *InMemStore) GetStoredResponse(
	_ context.Context,
	key string,
) (*StoredResponse, bool, error) {
	if s.withGetStoredResponseError {
		return nil,
			false,
			fmt.Errorf(
				"GetStoredResponse: %w",
				errors.New("get stored response error"), //nolint:err113 // this is a test store
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
				errors.New("unexpected response type"), //nolint:err113 // this is a test store
			)
	}

	// Check if expired
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
