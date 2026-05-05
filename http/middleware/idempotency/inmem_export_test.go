package idempotency

// Test-only knobs that flip the InMemStore into deliberate-error modes
// used by middleware tests. Defined in a *_test.go file so they do not
// appear in the production package surface.

func (s *InMemStore) withActiveStoreResponseError() {
	s.withStoreResponseError = true
}

func (s *InMemStore) withActiveGetStoredResponseError() {
	s.withGetStoredResponseError = true
}
