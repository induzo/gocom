package idempotency

import (
	"context"
	"errors"
	"sync"
)

var _ Store = &InMemStore{}

type InMemStore struct {
	mu        sync.RWMutex
	locks     sync.Map // key -> struct{}
	responses sync.Map // key -> *StoredResponse
}

// NewInMemStore initializes an in-memory store.
func NewInMemStore() *InMemStore {
	return &InMemStore{
		locks:     sync.Map{},
		responses: sync.Map{},
	}
}

func (s *InMemStore) TryLock(ctx context.Context, key string) (context.Context, context.CancelFunc, error) {
	if _, loaded := s.locks.LoadOrStore(key, struct{}{}); loaded {
		return ctx, nil, errors.New("key is already locked")
	}

	return ctx, func() {
		s.locks.Delete(key)
	}, nil

}

func (s *InMemStore) StoreResponse(_ context.Context, key string, resp *StoredResponse) error {
	if _, loaded := s.responses.LoadOrStore(key, resp); loaded {
		return errors.New("key already present")
	}

	return nil
}

func (s *InMemStore) GetStoredResponse(_ context.Context, key string) (*StoredResponse, bool, error) {
	resp, ok := s.responses.Load(key)
	if !ok || resp == nil {
		return nil, false, nil
	}

	storedResp, ok := resp.(*StoredResponse)
	if !ok {
		return nil, false, errors.New("unexpected response type")
	}

	return storedResp, true, nil
}
