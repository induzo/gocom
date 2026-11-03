package valkeydempotency

import (
	"context"
	"errors"
	"fmt"
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
		{
			name:         "error - ttl is sub-second",
			lockerOption: &valkeylock.LockerOption{},
			ttl:          500 * time.Millisecond,
			wantErr:      true,
		},
		{
			name:         "error - nil locker option",
			lockerOption: nil,
			ttl:          1 * time.Second,
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
				// Hold the lock from a separate context so the second
				// TryLock below fails because the key is held, not
				// because its context was cancelled.
				_, unlockPrevLock, errSR = storeValkey.TryLock(context.Background(), tt.key)
				if errSR != nil {
					t.Fatalf("TryLock() error: %v", errSR)
				}

				defer unlockPrevLock()
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

			if tt.wantErr && !errors.Is(err, idempotency.ErrLockHeld) {
				t.Errorf("held lock error %v should wrap idempotency.ErrLockHeld", err)
			}

			if unlock != nil {
				unlock()
			}
		})
	}
}

// TestStoreLock_StoreFailureIsNotLockHeld checks that a TryLock failure
// that is not "the key is held" (here: the locker has been closed) does
// not wrap idempotency.ErrLockHeld, so the middleware reports it as a
// store failure instead of an in-flight request.
func TestStoreLock_StoreFailureIsNotLockHeld(t *testing.T) {
	t.Parallel()

	storeValkey, errNS := NewStore(
		&valkeylock.LockerOption{
			ClientOption:   valkey.ClientOption{InitAddress: []string{testValkeyPortHost}},
			KeyMajority:    1,
			NoLoopTracking: true,
		},
		1*time.Second,
	)
	if errNS != nil {
		t.Fatalf("NewStore() error = %v", errNS)
	}

	if errC := storeValkey.Close(); errC != nil {
		t.Fatalf("Close() error = %v", errC)
	}

	_, unlock, err := storeValkey.TryLock(context.Background(), "trylock_closed_locker")
	if unlock != nil {
		unlock()
	}

	if err == nil {
		t.Fatal("TryLock() on a closed locker should fail")
	}

	if errors.Is(err, idempotency.ErrLockHeld) {
		t.Errorf("store failure %v must not wrap idempotency.ErrLockHeld", err)
	}

	if !errors.Is(err, valkeylock.ErrLockerClosed) {
		t.Errorf("error %v should keep the original cause valkeylock.ErrLockerClosed", err)
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
				if errSR := storeValkey.StoreResponse(
					context.Background(),
					tt.key,
					&idempotency.StoredResponse{},
				); errSR != nil {
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
				if errSR := storeValkey.StoreResponse(
					context.Background(),
					tt.key,
					sampleStoredResponse,
				); errSR != nil {
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
		60*time.Second,
	)
	if errNS != nil {
		b.Fatalf("NewStore() error = %v", errNS)
	}

	defer storeValkey.Close()

	resp := &idempotency.StoredResponse{
		StatusCode:  http.StatusOK,
		Header:      nil,
		Body:        []byte("body"),
		RequestHash: []byte("signature"),
	}

	i := 0

	for b.Loop() {
		key := fmt.Sprintf("bench-key-%d", i)
		i++

		ctx := context.Background()

		_, cancel, errL := storeValkey.TryLock(ctx, key)
		if errL != nil {
			b.Fatalf("TryLock: %v", errL)
		}

		if err := storeValkey.StoreResponse(ctx, key, resp); err != nil {
			cancel()
			b.Fatalf("StoreResponse: %v", err)
		}

		cancel()
	}
}

func TestWithLockerDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   time.Duration
		want time.Duration
	}{
		{name: "unset uses the store default", in: 0, want: defaultTryNextAfter},
		{name: "negative uses the store default", in: -time.Second, want: defaultTryNextAfter},
		{name: "explicit value is kept", in: 50 * time.Millisecond, want: 50 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			in := valkeylock.LockerOption{KeyMajority: 1, TryNextAfter: tt.in}
			got := withLockerDefaults(&in)

			if got.TryNextAfter != tt.want {
				t.Errorf("TryNextAfter = %v, want %v", got.TryNextAfter, tt.want)
			}

			if got.KeyMajority != in.KeyMajority {
				t.Errorf("KeyMajority changed: %d -> %d", in.KeyMajority, got.KeyMajority)
			}

			if in.TryNextAfter != tt.in {
				t.Errorf("caller option mutated: %v", in.TryNextAfter)
			}
		})
	}
}

func TestTTLIncorrectError_Error(t *testing.T) {
	t.Parallel()

	ttl := 500 * time.Millisecond
	err := &TTLIncorrectError{ttl: ttl}

	const want = "ttl must be at least 1s, got 500ms"
	if err.Error() != want {
		t.Errorf("expected error message to be %q, got %q", want, err.Error())
	}
}

func TestNewStore_NilLockerSentinel(t *testing.T) {
	t.Parallel()

	_, err := NewStore(nil, 1*time.Second)
	if !errors.Is(err, ErrNilLockerOption) {
		t.Errorf("expected ErrNilLockerOption, got %v", err)
	}
}
