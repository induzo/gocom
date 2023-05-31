package pgtest

import (
	"context"
	"embed"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/dockertest/v3"
	"golang.org/x/exp/slog"

	"github.com/induzo/gocom/database/pginit"
)

type DockertestWrapper struct {
	DockertestPool     *dockertest.Pool
	DockertestResource *dockertest.Resource
	ConnPool           *pgxpool.Pool
}

func New() *DockertestWrapper {
	var (
		pool     *dockertest.Pool
		resource *dockertest.Resource
		connPool *pgxpool.Pool
		err      error
	)

	pool, err = dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	if err = pool.Client.Ping(); err != nil {
		log.Fatalf(`Could not connect to docker: %s`, err)
	}

	resource, err = pool.Run("postgres", "14", []string{
		"POSTGRES_PASSWORD=postgres",
		"POSTGRES_USER=postgres",
		"POSTGRES_DB=datawarehouse",
		"listen_addresses = '*'",
	})
	if err != nil {
		log.Fatalf("Could not start resource: %s", err)
	}

	textHandler := slog.NewTextHandler(io.Discard, nil)
	logger := slog.New(textHandler)

	if errP := pool.Retry(func() error {
		pgi, errI := pginit.New(
			&pginit.Config{
				Host:         "localhost",
				Port:         strings.Split(getHostPort(resource, "5432/tcp"), ":")[1],
				User:         "postgres",
				Password:     "postgres",
				Database:     "datawarehouse",
				MaxConns:     1,
				MaxIdleConns: 1,
				MaxLifeTime:  1 * time.Minute,
			},
			pginit.WithLogger(logger, "request-id"),
			pginit.WithDecimalType(),
			pginit.WithUUIDType())
		if errI != nil {
			return fmt.Errorf("%w", errI)
		}

		var errC error
		connPool, errC = pgi.ConnPool(context.Background())
		if errC != nil {
			return fmt.Errorf("%w", errC)
		}

		if errPing := connPool.Ping(context.Background()); errPing != nil {
			return fmt.Errorf("%w", errPing)
		}

		return nil
	}); errP != nil {
		if errPurge := pool.Purge(resource); errPurge != nil {
			log.Fatalf("Could not purge resource: %s", errPurge)
		}

		log.Fatalf("Could not connect to docker: %s", errP)
	}

	return &DockertestWrapper{
		DockertestPool:     pool,
		DockertestResource: resource,
		ConnPool:           connPool,
	}
}

func (dw *DockertestWrapper) Purge() {
	dw.ConnPool.Close()

	if err := dw.DockertestPool.Purge(dw.DockertestResource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}
}

func (dw *DockertestWrapper) PrepareTestCaseDB(
	migrationFiles embed.FS, migrationPath string,
) (*pgxpool.Pool, error) {
	var (
		ctx    = context.Background()
		dbName = "test_" + strings.ReplaceAll(uuid.Must(uuid.NewV7()).String(), "-", "")
	)

	if dw.ConnPool == nil {
		return nil, &ConnPoolNotFoundError{}
	}

	_, err := dw.ConnPool.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", dbName))
	if err != nil {
		return nil, fmt.Errorf("failed to create testcase db: %w", err)
	}

	databaseURL := fmt.Sprintf(
		"postgres://postgres:%s@%s/%s?sslmode=disable",
		"postgres",
		getHostPort(dw.DockertestResource, "5432/tcp"),
		dbName,
	)

	if errMig := runMigrations(databaseURL, migrationFiles, migrationPath); errMig != nil {
		return nil, fmt.Errorf("could not run migrations: %w", err)
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("could not connect database: %w", err)
	}

	return pool, nil
}

func getHostPort(resource *dockertest.Resource, portID string) string {
	dockerURL := os.Getenv("DOCKER_HOST")
	if dockerURL == "" {
		hostAndPort := resource.GetHostPort(portID)
		hp := strings.Split(hostAndPort, ":")
		testRefHost := hp[0]
		testRefPort := hp[1]

		return testRefHost + ":" + testRefPort
	}

	u, err := url.Parse(dockerURL)
	if err != nil {
		panic(err)
	}

	return u.Hostname() + ":" + resource.GetPort(portID)
}

func runMigrations(dbURL string, migrationFiles embed.FS, migrationPath string) error {
	d, err := iofs.New(migrationFiles, migrationPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	mig, err := migrate.NewWithSourceInstance("iofs", d, dbURL)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	defer mig.Close()

	err = mig.Up()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}
