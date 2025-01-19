package idempotency

import (
	"context"
	"errors"
	"sync"
)

var _ Store = &InMemStore{}

type InMemStore struct {
	locks                      sync.Map // key -> struct{}
	responses                  sync.Map // key -> *StoredResponse
	withStoreResponseError     bool
	withGetStoredResponseError bool
}

// NewInMemStore initializes an in-memory store.
func NewInMemStore() *InMemStore {
	return &InMemStore{
		locks:     sync.Map{},
		responses: sync.Map{},
	}
}

func (s *InMemStore) withActiveStoreResponseError() {
	s.withStoreResponseError = true
}

func (s *InMemStore) withActiveGetStoredResponseError() {
	s.withGetStoredResponseError = true
}

func (s *InMemStore) TryLock(ctx context.Context, key string) (context.Context, context.CancelFunc, error) {
	if _, loaded := s.locks.LoadOrStore(key, struct{}{}); loaded {
		return ctx, nil, errors.New("key is already locked") //nolint:err113 // this is a test store
	}

	return ctx, func() {
		s.locks.Delete(key)
	}, nil
}

func (s *InMemStore) StoreResponse(_ context.Context, key string, resp *StoredResponse) error {
	if s.withStoreResponseError {
		return errors.New("store error") //nolint:err113 // this is a test store
	}

	if _, loaded := s.responses.LoadOrStore(key, resp); loaded {
		return errors.New("key already present") //nolint:err113 // this is a test store
	}

	return nil
}

func (s *InMemStore) GetStoredResponse(_ context.Context, key string) (*StoredResponse, bool, error) {
	if s.withGetStoredResponseError {
		return nil, false, errors.New("get stored response error") //nolint:err113 // this is a test store
	}

	resp, found := s.responses.Load(key)
	if !found || resp == nil {
		return nil, false, nil
	}

	storedResp, valid := resp.(*StoredResponse)
	if !valid {
		return nil, false, errors.New("unexpected response type") //nolint:err113 // this is a test store
	}

	return storedResp, true, nil
}
