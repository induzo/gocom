package idempotency

import (
	"context"
	"testing"
)

func TestNewInMemStore(t *testing.T) {
	// TestNewInMemStore tests the NewInMemStore function.
	t.Parallel()

	store := NewInMemStore()

	if store == nil {
		t.Error("NewInMemStore returned nil")
	}
}

func TestInMemStoreInsertInFlight(t *testing.T) {
	// TestInMemStoreInsert tests the InMemStore.Insert method.
	t.Parallel()

	store := NewInMemStore()

	tests := []struct {
		name    string
		key     string
		sig     []byte
		wantErr bool
	}{
		{
			name:    "key does not exist",
			key:     "key",
			sig:     []byte("signature"),
			wantErr: false,
		},
		{
			name:    "key exists",
			key:     "key",
			sig:     []byte("signature"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.InsertInFlight(context.Background(), tt.key, tt.sig)

			if err != nil && !tt.wantErr {
				t.Errorf("unexpected error: %v", err)
			}

			if err == nil && tt.wantErr {
				t.Error("expected error")
			}
		})
	}
}

func TestInMemStoreGetInFlightSignature(t *testing.T) {
	// TestInMemStoreGet tests the InMemStore.Get method.
	t.Parallel()

	store := NewInMemStore()
	store.InsertInFlight(context.Background(), "key", []byte("signature"))

	tests := []struct {
		name    string
		key     string
		sig     []byte
		ok      bool
		wantErr bool
	}{
		{
			name:    "key exists",
			key:     "key",
			sig:     []byte("signature"),
			ok:      true,
			wantErr: false,
		},
		{
			name:    "key does not exist",
			key:     "key2",
			sig:     nil,
			ok:      false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig, ok, err := store.GetInFlightSignature(context.Background(), tt.key)

			if err != nil && !tt.wantErr {
				t.Errorf("unexpected error: %v", err)
			}

			if err == nil && tt.wantErr {
				t.Error("expected error")
			}

			if ok != tt.ok {
				t.Errorf("got ok %v, want %v", ok, tt.ok)
			}

			if string(sig) != string(tt.sig) {
				t.Errorf("got sig %s, want %s", sig, tt.sig)
			}
		})
	}
}

func TestInMemStoreGetStoredResponse(t *testing.T) {
	// TestInMemStoreGet tests the InMemStore.Get method.
	t.Parallel()

	store := NewInMemStore()
	store.MarkComplete(context.Background(), "key", &StoredResponse{
		StatusCode:       200,
		Headers:          nil,
		Body:             []byte("body"),
		RequestSignature: []byte("signature"),
	})

	tests := []struct {
		name    string
		key     string
		resp    *StoredResponse
		ok      bool
		wantErr bool
	}{
		{
			name: "key exists",
			key:  "key",
			resp: &StoredResponse{
				StatusCode:       200,
				Headers:          nil,
				Body:             []byte("body"),
				RequestSignature: []byte("signature"),
			},
			ok:      true,
			wantErr: false,
		},
		{
			name:    "key does not exist",
			key:     "key2",
			resp:    nil,
			ok:      false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, ok, err := store.GetStoredResponse(context.Background(), tt.key)

			if err != nil && !tt.wantErr {
				t.Errorf("unexpected error: %v", err)
			}

			if err == nil && tt.wantErr {
				t.Error("expected error")
			}

			if ok != tt.ok {
				t.Errorf("got ok %v, want %v", ok, tt.ok)
			}

			if resp != nil {
				if resp.StatusCode != tt.resp.StatusCode {
					t.Errorf("got status code %d, want %d", resp.StatusCode, tt.resp.StatusCode)
				}

				if string(resp.Body) != string(tt.resp.Body) {
					t.Errorf("got body %s, want %s", resp.Body, tt.resp.Body)
				}

				if string(resp.RequestSignature) != string(tt.resp.RequestSignature) {
					t.Errorf("got request signature %s, want %s", resp.RequestSignature, tt.resp.RequestSignature)
				}
			}
		})
	}
}

func TestInMemStoreRemoveInFlight(t *testing.T) {
	// TestInMemStoreRemove tests the InMemStore.RemoveInFlight method.
	t.Parallel()

	store := NewInMemStore()
	store.InsertInFlight(context.Background(), "key", []byte("signature"))

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "key exists",
			key:     "key",
			wantErr: false,
		},
		{
			name:    "key does not exist",
			key:     "key2",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.RemoveInFlight(context.Background(), tt.key)

			if err != nil && !tt.wantErr {
				t.Errorf("unexpected error: %v", err)
			}

			if err == nil && tt.wantErr {
				t.Error("expected error")
			}
		})
	}
}
