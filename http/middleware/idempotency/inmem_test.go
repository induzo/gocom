package idempotency

import (
	"context"
	"net/http"
	"testing"
)

func TestNewInMemStore(t *testing.T) {
	t.Parallel()

	store := NewInMemStore()

	if store == nil {
		t.Error("NewInMemStore returned nil")
	}
}

func TestInMemStoreLock(t *testing.T) {
	t.Parallel()

	store := NewInMemStore()

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
				store.locks.Store(tt.key, struct{}{})
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
				StatusCode:       http.StatusOK,
				Headers:          nil,
				Body:             []byte("body"),
				RequestSignature: []byte("signature"),
			},

			wantErr: false,
		},
		{
			name: "key exists",
			key:  "doesKeyexist",
			resp: &StoredResponse{
				StatusCode:       http.StatusOK,
				Headers:          nil,
				Body:             []byte("body"),
				RequestSignature: []byte("signature"),
			},
			doesKeyExist: true,
			wantErr:      true,
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

			resp, ok := store.responses.Load(tt.key)
			if !ok || resp == nil {
				t.Errorf("response not stored for key %s", tt.key)
			}
		})
	}
}

func TestInMemStoreGetStoredResponse(t *testing.T) {
	t.Parallel()

	store := NewInMemStore()

	sampleStoredResponse := &StoredResponse{
		StatusCode:       http.StatusOK,
		Headers:          nil,
		Body:             []byte("body"),
		RequestSignature: []byte("signature"),
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
				store.responses.Store(tt.key, tt.storedResponse)
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
