package idempotency

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewInMemStore(t *testing.T) {
	t.Parallel()

	store := NewInMemStore()
	defer store.Close()

	if store == nil {
		t.Error("NewInMemStore returned nil")
	}
}

func TestInMemStoreLock(t *testing.T) {
	t.Parallel()

	store := NewInMemStore()
	t.Cleanup(store.Close)

	tests := []struct {
		name         string
		key          string
		sig          []byte
		doesKeyExist bool
		completed    bool
		wantErr      bool
	}{
		{
			name:    "key does not exist",
			key:     "keynothere",
			sig:     []byte("signature"),
			wantErr: false,
		},
		{
			name:         "key exists",
			key:          "doesKeyexist",
			doesKeyExist: true,
			sig:          []byte("signature"),
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.doesKeyExist {
				// Store a lock with future expiry
				store.locks.Store(tt.key, time.Now().Add(1*time.Hour))
			}

			_, cancel, err := store.TryLock(context.Background(), tt.key)

			if (err != nil) != tt.wantErr {
				t.Errorf("error expected %t, got %v", tt.wantErr, err)
			}

			if cancel != nil {
				cancel()

				if _, ok := store.locks.Load(tt.key); ok {
					t.Errorf("lock not removed for key %s", tt.key)
				}
			}
		})
	}
}

func TestInMemStoreStoreResponse(t *testing.T) {
	t.Parallel()

	store := NewInMemStore()
	t.Cleanup(store.Close)

	tests := []struct {
		name         string
		key          string
		resp         *StoredResponse
		doesKeyExist bool
		wantErr      bool
	}{
		{
			name: "key does not exist",
			key:  "keynothere",
			resp: &StoredResponse{
				StatusCode:  http.StatusOK,
				Header:      nil,
				Body:        []byte("body"),
				RequestHash: []byte("signature"),
			},

			wantErr: false,
		},
		{
			// StoreResponse now overwrites any prior entry for the key
			// rather than refusing, so that retries after a transient
			// failure can succeed.
			name: "key exists is overwritten",
			key:  "doesKeyexist",
			resp: &StoredResponse{
				StatusCode:  http.StatusOK,
				Header:      nil,
				Body:        []byte("body"),
				RequestHash: []byte("signature"),
			},
			doesKeyExist: true,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.doesKeyExist {
				store.responses.Store(tt.key, struct{}{})
			}

			err := store.StoreResponse(context.Background(), tt.key, tt.resp)
			if (err != nil) != tt.wantErr {
				t.Errorf("error expected %t, got %v", tt.wantErr, err)
			}

			if tt.wantErr {
				return
			}

			value, ok := store.responses.Load(tt.key)
			if !ok || value == nil {
				t.Errorf("response not stored for key %s", tt.key)
			}

			if _, ok := value.(*storedEntry); !ok {
				t.Errorf("stored value type = %T, want *storedEntry (overwrite expected)", value)
			}
		})
	}
}

// TestInMemStore_TryLock_ExpiredRaceSingleWinner exercises the CAS path in
// TryLock when many goroutines simultaneously observe an expired lock.
// Exactly one of them should succeed; the rest must see "key is already
// locked".
func TestInMemStore_TryLock_ExpiredRaceSingleWinner(t *testing.T) {
	t.Parallel()

	store := NewInMemStore()
	t.Cleanup(store.Close)

	const key = "race-key"

	// Seed an already-expired lock so all callers race through the CAS path.
	store.locks.Store(key, time.Now().Add(-time.Hour))

	const goroutines = 64

	var (
		wg       sync.WaitGroup
		winners  atomic.Int32
		started  = make(chan struct{})
		ctx      = context.Background()
		successC = make(chan func(), goroutines)
	)

	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()

			<-started

			_, cancel, err := store.TryLock(ctx, key)
			if err != nil {
				return
			}

			winners.Add(1)

			successC <- cancel
		}()
	}

	close(started)
	wg.Wait()
	close(successC)

	if got := winners.Load(); got != 1 {
		t.Errorf("winners = %d, want exactly 1 with CAS-protected expiry", got)
	}

	for cancel := range successC {
		cancel()
	}
}

func TestInMemStore_Close_Idempotent(t *testing.T) {
	t.Parallel()

	store := NewInMemStore()
	store.Close()
	store.Close() // must not panic on second call.
}

func TestInMemStoreGetStoredResponse(t *testing.T) {
	t.Parallel()

	store := NewInMemStore()
	t.Cleanup(store.Close)

	sampleStoredResponse := &StoredResponse{
		StatusCode:  http.StatusOK,
		Header:      nil,
		Body:        []byte("body"),
		RequestHash: []byte("signature"),
	}

	tests := []struct {
		name           string
		key            string
		storedResponse any
		expectedResp   *StoredResponse
		ok             bool
		wantErr        bool
	}{
		{
			name:           "key exists",
			key:            "key",
			storedResponse: sampleStoredResponse,
			expectedResp:   sampleStoredResponse,
			ok:             true,
			wantErr:        false,
		},
		{
			name:           "key does not exist",
			key:            "key2",
			storedResponse: nil,
			expectedResp:   nil,
			ok:             false,
			wantErr:        false,
		},
		{
			name:           "unexpected response type stored",
			key:            "key3",
			storedResponse: 1,
			expectedResp:   nil,
			ok:             false,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.storedResponse != nil {
				// For testing "unexpected response type", store the value directly
				if respPtr, ok := tt.storedResponse.(*StoredResponse); ok {
					entry := &storedEntry{
						response:  respPtr,
						expiresAt: time.Now().Add(1 * time.Hour),
					}
					store.responses.Store(tt.key, entry)
				} else {
					// Store invalid type directly for testing error handling
					store.responses.Store(tt.key, tt.storedResponse)
				}
			}

			resp, ok, err := store.GetStoredResponse(context.Background(), tt.key)

			if (err != nil) != tt.wantErr {
				t.Errorf("error expected %t, got %v", tt.wantErr, err)

				return
			}

			if ok != tt.ok {
				t.Errorf("got ok %v, want %v", ok, tt.ok)
			}

			if resp != tt.expectedResp {
				t.Errorf("got resp %v, want %v", resp, sampleStoredResponse)
			}
		})
	}
}
