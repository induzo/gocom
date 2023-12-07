package pginit

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestJSONRowToAddrOfStruct(t *testing.T) {
	t.Parallel()

	type testStruct struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	tests := []struct {
		name    string
		json    string
		want    *testStruct
		wantErr bool
	}{
		{
			name: "valid json",
			json: `{"id": 10, "name": "dodo"}`,
			want: &testStruct{
				ID:   10,
				Name: "dodo",
			},
		},
		{
			name:    "valid json",
			json:    `{"id": 10, "name": "dodo"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			pgi, err := New(
				connStr,
			)
			if err != nil {
				t.Error("expected no error")

				return
			}

			db, err := pgi.ConnPool(ctx)
			if err != nil {
				t.Error("expected no error")

				return
			}

			defer db.Close()

			err = db.Ping(ctx)
			if err != nil {
				t.Error("expected no error")
			}

			rows, errQ := db.Query(context.Background(), fmt.Sprintf(`select '%s'`, tt.json))
			if errQ != nil {
				t.Errorf("expected no err: %s", errQ)

				return
			}

			got, errA := pgx.CollectExactlyOneRow(rows, JSONRowToAddrOfStruct[testStruct])
			if (errA != nil) != tt.wantErr {
				t.Errorf("could not collect rows: %v", errA)

				return
			}

			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("JSONRowToAddrOfStruct() = %v, want %v", got, tt.want)
			}
		})
	}
}
