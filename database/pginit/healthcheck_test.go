package pginit

import (
	"context"
	"testing"
)

func TestConnPoolHealthCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		closeDB bool
		wantErr bool
	}{
		{
			name:    "happy path",
			closeDB: false,
			wantErr: false,
		},
		{
			name:    "conn closed",
			closeDB: true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pgi, err := New(connStr)
			if err != nil {
				t.Errorf("unexpected error in test (%v)", err)
			}

			db, err := pgi.ConnPool(context.Background())
			if err != nil {
				t.Errorf("unexpected error in test (%v)", err)
			}

			if tt.closeDB {
				db.Close()
			} else {
				defer db.Close()
			}

			healthCheck := ConnPoolHealthCheck(db)

			if err := healthCheck(context.Background()); (err != nil) != tt.wantErr {
				t.Errorf("expected err: %v, got err: %v", tt.wantErr, (err != nil))
			}
		})
	}
}
