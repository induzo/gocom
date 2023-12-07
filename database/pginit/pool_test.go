package pginit

import (
	"context"
	"errors"
	"flag"
	"io"
	"log"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/goleak"
)

var connStr string //nolint:gochecknoglobals // test code

func TestMain(m *testing.M) {
	ctx := context.Background()

	postgresContainer, errR := postgres.RunContainer(
		ctx,
		testcontainers.WithImage("docker.io/postgres:16-alpine"),
		postgres.WithDatabase("datawarehouse"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
	if errR != nil {
		log.Fatalf("could not create container: %v", errR)
	}

	// Clean up the container
	defer func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			panic(err)
		}
	}()

	var err error

	connStr, err = postgresContainer.ConnectionString(ctx, "sslmode=disable", "application_name=test")
	if err != nil {
		log.Fatalf("could not retrieve conn string: %v", err)
	}

	leak := flag.Bool("leak", false, "use leak detector")
	flag.Parse()

	if *leak {
		goleak.VerifyTestMain(m, goleak.IgnoreAnyFunction("github.com/testcontainers/testcontainers-go.(*Reaper).Connect.func1"))

		return
	}

	os.Exit(m.Run())
}

func TestConnPool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		connString          string
		wantMinConns        int32
		wantMaxConns        int32
		wantMaxConnLifetime time.Duration
		wantErr             bool
	}{
		{
			name:                "expecting no error with default connection setting",
			connString:          connStr,
			wantMinConns:        0,
			wantMaxConnLifetime: 60 * time.Minute,
			wantErr:             false,
		},
		{
			name:       "expecting error with wrong user setting",
			connString: "postgres://postgres:postgres@localhost:5432/testbadconn?sslmode=disable",
			wantErr:    true,
		},
		{
			name:       "expecting error with wrong conn string",
			connString: "postg:/",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.TODO()

			textHandler := slog.NewTextHandler(io.Discard, nil)
			logger := slog.New(textHandler)

			pgi, err := New(tt.connString, WithLogger(logger, ""))
			if err != nil {
				if !tt.wantErr {
					t.Errorf("expect no err but err returned: %s", err)
				}

				return
			}

			db, err := pgi.ConnPool(ctx)
			if tt.wantErr && err == nil {
				t.Errorf("expects err but nil returned")
			}

			if err != nil {
				if !tt.wantErr {
					t.Errorf("expect no err but err returned: %s", err)
				}

				return
			}

			defer db.Close()

			if err := db.Ping(ctx); err != nil {
				t.Errorf("cannot ping db: %s", err)
			}
			if db.Config().MinConns != tt.wantMinConns {
				t.Errorf("expected (%v) but got (%v)", tt.wantMinConns, db.Config().MinConns)
			}
			if db.Config().MaxConnLifetime != tt.wantMaxConnLifetime {
				t.Errorf("expected (%v) but got (%v)", tt.wantMaxConnLifetime, db.Config().MaxConnLifetime)
			}
		})
	}
}

func TestConnPoolWithLogger(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
	}{
		{
			name: "level debug",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			textHandler := slog.NewTextHandler(io.Discard, nil)
			logger := slog.New(textHandler)

			pgi, err := New(
				connStr,
				WithLogger(logger, "request-id"),
				WithDecimalType(),
				WithUUIDType(),
			)
			if err != nil {
				t.Error("expected no error")
			}

			db, err := pgi.ConnPool(ctx)
			if err != nil {
				t.Error("expected no error")
			}

			defer db.Close()

			if err := db.Ping(ctx); err != nil {
				t.Error("expected no error")
			}

			if db.Config().ConnConfig.Tracer == nil {
				t.Error("expected logger not nil")
			}

			if _, err := db.Exec(ctx, "SELECT * FROM ERROR"); err == nil {
				t.Error("expected return error")
			}
		})
	}
}

func TestConnPoolWithTracer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
	}{
		{
			name: "level debug",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			pgi, err := New(
				connStr,
				WithTracer(),
			)
			if err != nil {
				t.Error("expected no error")
			}

			db, err := pgi.ConnPool(ctx)
			if err != nil {
				t.Error("expected no error")
			}

			defer db.Close()

			if err := db.Ping(ctx); err != nil {
				t.Error("expected no error")
			}

			if db.Config().ConnConfig.Tracer == nil {
				t.Error("expected logger not nil")
			}

			if _, err := db.Exec(ctx, "SELECT * FROM ERROR"); err == nil {
				t.Error("expected return error")
			}
		})
	}
}

func TestConnPool_WithCustomDataTypes(t *testing.T) {
	t.Parallel()

	textHandler := slog.NewTextHandler(io.Discard, nil)
	logger := slog.New(textHandler)

	tests := []struct {
		name             string
		opts             []Option
		expectErrDecimal bool
		expectErrUUID    bool
	}{
		{
			name: "decimal + uuid",
			opts: []Option{
				WithLogger(logger, "request-id"),
				WithDecimalType(),
				WithUUIDType(),
			},
			expectErrDecimal: false,
			expectErrUUID:    false,
		},
		{
			name: "uuid + decimal",
			opts: []Option{
				WithLogger(logger, "request-id"),
				WithUUIDType(),
				WithDecimalType(),
			},
			expectErrDecimal: false,
			expectErrUUID:    false,
		},
		{
			name: "decimal",
			opts: []Option{
				WithLogger(logger, "request-id"),
				WithDecimalType(),
			},
			expectErrDecimal: false,
			expectErrUUID:    true,
		},
		{
			name: "uuid",
			opts: []Option{
				WithLogger(logger, "request-id"),
				WithUUIDType(),
			},
			expectErrDecimal: true,
			expectErrUUID:    false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			pgi, err := New(
				connStr,
				tt.opts...,
			)
			if err != nil {
				t.Error("expected no error")
			}

			db, err := pgi.ConnPool(ctx)
			if err != nil {
				t.Error("expected no error")
			}

			defer db.Close()

			err = db.Ping(ctx)
			if err != nil {
				t.Error("expected no error")
			}

			var d decimal.Decimal
			err = db.QueryRow(context.Background(), "select 10.98::numeric").Scan(&d)
			if err != nil && !tt.expectErrDecimal {
				t.Errorf("expected no err: %s", err)
			}

			var u uuid.UUID
			err = db.QueryRow(context.Background(), "select 'b7202eb0-5bf0-475d-8ee2-d3d2c168a5d5'::uuid").Scan(&u)
			if err != nil && !tt.expectErrUUID {
				t.Errorf("expected no err: %s", err)
			}
		})
	}
}

func TestConnPoolWithCustomTypes_CRUD(t *testing.T) {
	t.Parallel()

	tenPointEight, _ := decimal.NewFromString("10.888888888888")
	eleven, _ := decimal.NewFromString("11.00")

	ctx := context.Background()

	tests := []struct {
		name string
	}{
		{
			name: "CRUD operation with custom type uuid and decimal",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			textHandler := slog.NewTextHandler(io.Discard, nil)
			logger := slog.New(textHandler)

			pgi, err := New(
				connStr,
				WithLogger(logger, "request-id"),
				WithDecimalType(),
				WithUUIDType(),
			)
			if err != nil {
				t.Errorf("expected no error but got: %v", err)
			}

			pool, err := pgi.ConnPool(ctx)
			if err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
			defer pool.Close()

			conn, err := pool.Acquire(ctx)
			if err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
			defer conn.Release()

			tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
			if err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
			defer tx.Rollback(ctx)

			_, err = tx.Exec(ctx, "CREATE TABLE IF NOT EXISTS uuid_decimal(uuid uuid, price numeric, PRIMARY KEY (uuid))")
			if err != nil {
				t.Errorf("expected no error but got: %v", err)
			}

			// create
			row := tx.QueryRow(ctx, "INSERT INTO uuid_decimal(uuid, price) VALUES('b7202eb0-5bf0-475d-8ee2-d3d2c168a5d5', 10.888888888888) RETURNING uuid, price")
			r := struct {
				uuid  uuid.UUID
				price decimal.Decimal
			}{}
			if err := row.Scan(&r.uuid, &r.price); err != nil { //nolint:govet // inline err is within scope
				t.Errorf("expected no error but got: %v, (%+v)", err, row)
			}

			if r.uuid.String() != "b7202eb0-5bf0-475d-8ee2-d3d2c168a5d5" {
				t.Errorf("expected %s but got: %s", "b7202eb0-5bf0-475d-8ee2-d3d2c168a5d5", r.uuid.String())
			}

			if r.price.Cmp(tenPointEight) != 0 {
				t.Errorf("expected %s but got: %s", tenPointEight.String(), r.price.String())
			}

			// read
			rows, err := tx.Query(ctx, "SELECT * FROM uuid_decimal")
			if err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
			defer rows.Close()
			var results []struct {
				uuid  uuid.UUID
				price decimal.Decimal
			}
			for rows.Next() {
				r := struct { //nolint:govet // r is within loop scope
					uuid  uuid.UUID
					price decimal.Decimal
				}{}
				if err := rows.Scan(&r.uuid, &r.price); err != nil {
					t.Errorf("expected no error but got: %v", err)
				}

				if r.uuid.String() != "b7202eb0-5bf0-475d-8ee2-d3d2c168a5d5" {
					t.Errorf("expected %s but got: %s", "b7202eb0-5bf0-475d-8ee2-d3d2c168a5d5", r.uuid.String())
				}

				if r.price.Cmp(tenPointEight) != 0 {
					t.Errorf("expected %s but got: %s", tenPointEight.String(), r.price.String())
				}

				results = append(results, r)
			}
			if len(results) != 1 {
				t.Errorf("expected 1 result but got: %v", len(results))
			}
			// update
			row = tx.QueryRow(ctx, "UPDATE uuid_decimal SET price = 11.00 WHERE uuid = $1 RETURNING uuid, price", "b7202eb0-5bf0-475d-8ee2-d3d2c168a5d5")
			if err := row.Scan(&r.uuid, &r.price); err != nil {
				t.Errorf("expected no error but got: %v, (%+v)", err, row)
			}
			if r.price.Cmp(eleven) != 0 {
				t.Errorf("expected 11.00 but got %s", r.price.String())
			}

			// delete
			row = tx.QueryRow(ctx, "DELETE FROM uuid_decimal WHERE uuid = $1 RETURNING uuid, price", "b7202eb0-5bf0-475d-8ee2-d3d2c168a5d5")
			if err := row.Scan(&r.uuid, &r.price); err != nil {
				t.Errorf("expected no error but got: %v, (%+v)", err, row)
			}
			if r.uuid.String() != "b7202eb0-5bf0-475d-8ee2-d3d2c168a5d5" {
				t.Error("inserted data doesn't match with input")
			}
			row = tx.QueryRow(ctx, "SELECT * FROM uuid_decimal WHERE uuid = $1", "b7202eb0-5bf0-475d-8ee2-d3d2c168a5d5")
			if err := row.Scan(&r.uuid, &r.price); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				t.Errorf("expected no error but got: %v, (%+v)", err, row)
			}
		})
	}
}

func BenchmarkConnPool(b *testing.B) {
	for i := 0; i <= b.N; i++ {
		ctx := context.Background()

		textHandler := slog.NewTextHandler(io.Discard, nil)
		logger := slog.New(textHandler)

		b.StartTimer()

		pgi, _ := New(
			connStr,
			WithLogger(logger, "request-id"),
			WithDecimalType(),
			WithUUIDType(),
		)

		pgi.ConnPool(ctx)

		b.StopTimer()
	}
}
