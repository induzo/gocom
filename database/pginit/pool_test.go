package pginit

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
	"github.com/ory/dockertest/v3"
	"github.com/shopspring/decimal"
	"go.uber.org/goleak"
	"golang.org/x/exp/slog"
)

var testHost, testPort string //nolint:gochecknoglobals // postgres dockertest info

func TestMain(m *testing.M) {
	dockerPool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	resource, err := dockerPool.Run("postgres", "14", []string{
		"POSTGRES_PASSWORD=postgres",
		"POSTGRES_USER=postgres",
		"POSTGRES_DB=datawarehouse",
		"listen_addresses = '*'",
	})
	if err != nil {
		log.Fatalf("Could not start resource: %s", err)
	}

	databaseURL := fmt.Sprintf("postgres://postgres:%s@%s/datawarehouse?sslmode=disable", "postgres", getHostPort(resource, "5432/tcp"))

	if err := dockerPool.Retry(func() error {
		ctx := context.Background()
		db, err := pgx.Connect(ctx, databaseURL)
		if err != nil {
			return fmt.Errorf("pgx connect: %w", err)
		}
		if err := db.Ping(ctx); err != nil {
			return fmt.Errorf("ping: %w", err)
		}

		return nil
	}); err != nil {
		log.Fatalf("Could not connect to docker(%s): %s", databaseURL, err)
	}

	leak := flag.Bool("leak", false, "use leak detector")
	flag.Parse()

	code := m.Run()

	if *leak {
		if code == 0 {
			if err := goleak.Find(); err != nil {
				log.Fatalf("goleak: Errors on successful test run: %v\n", err) //nolint:revive // this is a test

				code = 1
			}
		}
	}

	// You can't defer this because os.Exit doesn't care for defer
	if err := dockerPool.Purge(resource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}

	os.Exit(code)
}

func getHostPort(resource *dockertest.Resource, id string) string {
	dockerURL := os.Getenv("DOCKER_HOST")
	if dockerURL == "" {
		hostAndPort := resource.GetHostPort("5432/tcp")
		hp := strings.Split(hostAndPort, ":")
		testHost = hp[0]
		testPort = hp[1]

		return testHost + ":" + testPort
	}

	u, err := url.Parse(dockerURL)
	if err != nil {
		panic(err)
	}

	testHost = u.Hostname()
	testPort = resource.GetPort(id)

	return u.Hostname() + ":" + resource.GetPort(id)
}

func TestConnPool(t *testing.T) {
	t.Parallel()

	type args struct {
		Config Config
	}

	tests := []struct {
		name       string
		args       args
		wantConfig Config
		wantErr    bool
	}{
		{
			name: "expecting no error with default connection setting",
			args: args{
				Config{
					Host:     testHost,
					Port:     testPort,
					User:     "postgres",
					Password: "postgres",
					Database: "datawarehouse",
				},
			},
			wantConfig: Config{
				MaxConns:     25,
				MaxIdleConns: 25,
				MaxLifeTime:  5 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "expecting no error with custom connection setting",
			args: args{
				Config{
					Host:         testHost,
					Port:         testPort,
					User:         "postgres",
					Password:     "postgres",
					Database:     "datawarehouse",
					MaxConns:     15,
					MaxIdleConns: 10,
					MaxLifeTime:  10 * time.Minute,
				},
			},
			wantConfig: Config{
				MaxConns:     15,
				MaxIdleConns: 10,
				MaxLifeTime:  10 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "expecting error with wrong user setting",
			args: args{
				Config{
					Host:     testHost,
					Port:     testPort,
					User:     "wrong",
					Password: "postgres",
					Database: "datawarehouse",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.TODO()

			textHandler := slog.NewTextHandler(io.Discard)
			logger := slog.New(textHandler)

			pgi, err := New(&tt.args.Config, WithLogger(logger, ""))
			if err != nil {
				t.Errorf("unexpected error in test (%v)", err)
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
			if db.Config().MaxConns != tt.wantConfig.MaxConns {
				t.Errorf("expected (%v) but got (%v)", tt.wantConfig.MaxConns, db.Config().MaxConns)
			}
			if db.Config().MaxConnLifetime != tt.wantConfig.MaxLifeTime {
				t.Errorf("expected (%v) but got (%v)", tt.wantConfig.MaxLifeTime, db.Config().MaxConnLifetime)
			}
			if db.Config().MinConns != tt.wantConfig.MaxIdleConns {
				t.Errorf("expected (%v) but got (%v)", tt.wantConfig.MaxIdleConns, db.Config().MinConns)
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

			textHandler := slog.NewTextHandler(io.Discard)
			logger := slog.New(textHandler)

			pgi, err := New(
				&Config{
					Host:     testHost,
					Port:     testPort,
					User:     "postgres",
					Password: "postgres",
					Database: "datawarehouse",
					MaxConns: 2,
				},
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

func TestConnPool_WithCustomDataTypes(t *testing.T) {
	t.Parallel()

	textHandler := slog.NewTextHandler(io.Discard)
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
				&Config{
					Host:     testHost,
					Port:     testPort,
					User:     "postgres",
					Password: "postgres",
					Database: "datawarehouse",
					MaxConns: 2,
				},
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
			err = db.QueryRow(context.Background(), "select 10.98").Scan(d)
			if err != nil && !tt.expectErrDecimal {
				t.Errorf("expected no err: %s", err)
			}

			var u uuid.UUID
			err = db.QueryRow(context.Background(), "select 'b7202eb0-5bf0-475d-8ee2-d3d2c168a5d5'").Scan(u)
			if err != nil && !tt.expectErrUUID {
				t.Errorf("expected no err: %s", err)
			}
		})
	}
}

func TestConnPoolWithCustomTypes_CRUD(t *testing.T) {
	t.Parallel()

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

			textHandler := slog.NewTextHandler(io.Discard)
			logger := slog.New(textHandler)

			pgi, err := New(&Config{
				Host:     testHost,
				Port:     testPort,
				User:     "postgres",
				Password: "postgres",
				Database: "datawarehouse",
			},
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
			row := tx.QueryRow(ctx, "INSERT INTO uuid_decimal(uuid, price) VALUES('b7202eb0-5bf0-475d-8ee2-d3d2c168a5d5', 10.988888888889) RETURNING uuid, price")
			r := struct {
				uuid  uuid.UUID
				price decimal.Decimal
			}{}
			if err := row.Scan(&r.uuid, &r.price); err != nil { //nolint:govet // inline err is within scope
				t.Errorf("expected no error but got: %v, (%+v)", err, row)
			}
			if r.uuid.String() != "b7202eb0-5bf0-475d-8ee2-d3d2c168a5d5" || r.price.Cmp(decimal.New(10988888888889, 12)) != 0 {
				t.Error("inserted data doesn't match with input")
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
				if r.uuid.String() != "b7202eb0-5bf0-475d-8ee2-d3d2c168a5d5" || r.price.Cmp(decimal.New(10988888888889, 12)) != 0 {
					t.Error("inserted data doesn't match with input")
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
			if r.price.Cmp(decimal.New(1100, 2)) != 0 {
				t.Errorf("expected 11.00 but got %+v", r)
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

		textHandler := slog.NewTextHandler(io.Discard)
		logger := slog.New(textHandler)

		b.StartTimer()

		pgi, _ := New(
			&Config{
				Host:     testHost,
				Port:     testPort,
				User:     "postgres",
				Password: "postgres",
				Database: "datawarehouse",
			},
			WithLogger(logger, "request-id"),
			WithDecimalType(),
			WithUUIDType(),
		)

		pgi.ConnPool(ctx)

		b.StopTimer()
	}
}
