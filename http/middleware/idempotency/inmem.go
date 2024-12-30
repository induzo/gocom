package idempotency

import (
	"context"
	"errors"
	"sync"
)

type inMemStore struct {
	mu        sync.RWMutex
	inFlight  map[string][]byte // key -> request signature
	responses map[string]*StoredResponse
}

// NewInMemStore initializes an in-memory store.
func NewInMemStore() *inMemStore {
	return &inMemStore{
		inFlight:  make(map[string][]byte),
		responses: make(map[string]*StoredResponse),
	}
}

func (s *inMemStore) InsertInFlight(ctx context.Context, key string, requestSignature []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If we already have a marker or a stored response, we should decide
	// how to handle. Typically you check outside of InsertInFlight before calling it.
	if _, ok := s.inFlight[key]; ok {
		return errors.New("already in-flight")
	}
	if _, ok := s.responses[key]; ok {
		return errors.New("already completed")
	}

	s.inFlight[key] = requestSignature
	return nil
}

func (s *inMemStore) MarkComplete(ctx context.Context, key string, resp *StoredResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// We assume the request was in-flight, so we remove that marker
	// and store the final response.
	delete(s.inFlight, key)
	s.responses[key] = resp
	return nil
}

func (s *inMemStore) GetInFlightSignature(ctx context.Context, key string) ([]byte, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sig, ok := s.inFlight[key]
	return sig, ok, nil
}

func (s *inMemStore) GetStoredResponse(ctx context.Context, key string) (*StoredResponse, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resp, ok := s.responses[key]
	return resp, ok, nil
}

func (s *inMemStore) RemoveInFlight(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.inFlight, key)
	return nil
}
