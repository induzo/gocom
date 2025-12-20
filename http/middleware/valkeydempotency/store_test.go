package valkeydempotency

import (
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/valkey-io/valkey-go"
	"github.com/valkey-io/valkey-go/valkeylock"

	"github.com/induzo/gocom/http/middleware/idempotency"
)

func TestNewStore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		lockerOption *valkeylock.LockerOption
		ttl          time.Duration
		wantErr      bool
	}{
		{
			name: "success",
			lockerOption: &valkeylock.LockerOption{
				ClientOption:   valkey.ClientOption{InitAddress: []string{testValkeyPortHost}},
				KeyMajority:    1,    // Use KeyMajority=1 if you have only one Valkey instance. Also make sure that all your `Locker`s share the same KeyMajority.
				NoLoopTracking: true, // Enable this to have better performance if all your Valkey are >= 7.0.5.
			},
			ttl: 1 * time.Second,
		},
		{
			name:         "error - ttl is zero",
			lockerOption: &valkeylock.LockerOption{},
			ttl:          0,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sto, err := NewStore(tt.lockerOption, tt.ttl)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewStore() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				defer sto.Close()
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("NewStore() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestStoreLock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		key      string
		isLocked bool
		wantErr  bool
	}{
		{
			name:     "not locked",
			key:      "trylock_nolock",
			isLocked: false,
			wantErr:  false,
		},
		{
			name:     "locked in",
			key:      "trylock_lockinplace",
			isLocked: true,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			storeValkey, errNS := NewStore(
				&valkeylock.LockerOption{
					ClientOption:   valkey.ClientOption{InitAddress: []string{testValkeyPortHost}},
					KeyMajority:    1,    // Use KeyMajority=1 if you have only one Valkey instance. Also make sure that all your `Locker`s share the same KeyMajority.
					NoLoopTracking: true, // Enable this to have better performance if all your Valkey are >= 7.0.5.
				},
				1*time.Second,
			)
			if errNS != nil {
				t.Fatalf("NewStore() error = %v", errNS)
			}

			defer storeValkey.Close()

			ctx := context.Background()

			var (
				unlockPrevLock context.CancelFunc
				errSR          error
			)

			if tt.isLocked {
				ctx, unlockPrevLock, errSR = storeValkey.TryLock(ctx, tt.key)
				if errSR != nil {
					t.Fatalf("StoreResponse() error: %v", errSR)
				}

				unlockPrevLock()
			}

			var (
				unlock context.CancelFunc
				err    error
			)

			_, unlock, err = storeValkey.TryLock(ctx, tt.key)

			if (err != nil) != tt.wantErr {
				t.Errorf("error expected %t, got %v", tt.wantErr, err)

				return
			}

			if unlock != nil {
				unlock()
			}

			if unlockPrevLock != nil {
				unlockPrevLock()
			}
		})
	}
}

func TestStoreStoreResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		key          string
		resp         *idempotency.StoredResponse
		doesKeyExist bool
		wantErr      bool
	}{
		{
			name: "key does not exist",
			key:  "keynothere",
			resp: &idempotency.StoredResponse{
				StatusCode:  http.StatusOK,
				Header:      nil,
				Body:        []byte("body"),
				RequestHash: []byte("signature"),
			},
			wantErr: false,
		},
		{
			name: "key exists",
			key:  "doesKeyexist",
			resp: &idempotency.StoredResponse{
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

			storeValkey, errNS := NewStore(
				&valkeylock.LockerOption{
					ClientOption:   valkey.ClientOption{InitAddress: []string{testValkeyPortHost}},
					KeyMajority:    1,    // Use KeyMajority=1 if you have only one Valkey instance. Also make sure that all your `Locker`s share the same KeyMajority.
					NoLoopTracking: true, // Enable this to have better performance if all your Valkey are >= 7.0.5.
				},
				1*time.Second,
			)
			if errNS != nil {
				t.Fatalf("NewStore() error = %v", errNS)
			}

			defer storeValkey.Close()

			if tt.doesKeyExist {
				if errSR := storeValkey.StoreResponse(context.Background(), tt.key, &idempotency.StoredResponse{}); errSR != nil {
					t.Fatalf("StoreResponse() error = %v", errSR)
				}
			}

			err := storeValkey.StoreResponse(context.Background(), tt.key, tt.resp)
			if (err != nil) != tt.wantErr {
				t.Errorf("error expected %t, got %v", tt.wantErr, err)
			}

			if tt.wantErr {
				return
			}

			resp, ok, _ := storeValkey.GetStoredResponse(context.Background(), tt.key)
			if !ok || resp == nil {
				t.Errorf("response not stored for key %s", tt.key)
			}
		})
	}
}

func TestStoreGetStoredResponse(t *testing.T) {
	t.Parallel()

	sampleStoredResponse := &idempotency.StoredResponse{
		StatusCode:  http.StatusOK,
		Header:      nil,
		Body:        []byte("body"),
		RequestHash: []byte("signature"),
	}

	tests := []struct {
		name           string
		key            string
		storedResponse any
		expectedResp   *idempotency.StoredResponse
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			storeValkey, errNS := NewStore(
				&valkeylock.LockerOption{
					ClientOption:   valkey.ClientOption{InitAddress: []string{testValkeyPortHost}},
					KeyMajority:    1,    // Use KeyMajority=1 if you have only one Valkey instance. Also make sure that all your `Locker`s share the same KeyMajority.
					NoLoopTracking: true, // Enable this to have better performance if all your Valkey are >= 7.0.5.
				},
				100*time.Second,
			)
			if errNS != nil {
				t.Fatalf("NewStore() error = %v", errNS)
			}

			defer storeValkey.Close()

			if tt.storedResponse != nil {
				if errSR := storeValkey.StoreResponse(context.Background(), tt.key, sampleStoredResponse); errSR != nil {
					t.Fatalf("StoreResponse() error = %v", errSR)
				}
			}

			resp, ok, err := storeValkey.GetStoredResponse(context.Background(), tt.key)

			if (err != nil) != tt.wantErr {
				t.Errorf("error expected %t, got %v", tt.wantErr, err)

				return
			}

			if ok != tt.ok {
				t.Errorf("got ok %v, want %v", ok, tt.ok)
			}

			if !reflect.DeepEqual(resp, tt.expectedResp) {
				t.Errorf("got resp %v, want %v", resp, sampleStoredResponse)
			}
		})
	}
}

// bench
func BenchmarkStoreStoreResponse(b *testing.B) {
	b.ReportAllocs()

	storeValkey, errNS := NewStore(
		&valkeylock.LockerOption{
			ClientOption:   valkey.ClientOption{InitAddress: []string{testValkeyPortHost}},
			KeyMajority:    1,    // Use KeyMajority=1 if you have only one Valkey instance. Also make sure that all your `Locker`s share the same KeyMajority.
			NoLoopTracking: true, // Enable this to have better performance if all your Valkey are >= 7.0.5.
		},
		1*time.Second,
	)
	if errNS != nil {
		b.Fatalf("NewStore() error = %v", errNS)
	}

	defer storeValkey.Close()

	for b.Loop() {
		ctx := context.Background()

		ctx, cancel, _ := storeValkey.TryLock(ctx, "key")

		storeValkey.StoreResponse(ctx, "key", &idempotency.StoredResponse{
			StatusCode:  http.StatusOK,
			Header:      nil,
			Body:        []byte("body"),
			RequestHash: []byte("signature"),
		})

		cancel()
	}
}

func TestTTLIncorrectError_Error(t *testing.T) {
	t.Parallel()

	ttl := 1 * time.Second
	err := &TTLIncorrectError{ttl: ttl}

	if err.Error() != "ttl must be greater than 0, got 1s" {
		t.Errorf(
			"expected error message to be 'ttl must be greater than 0, got 1s', got '%s'",
			err.Error(),
		)
	}
}
