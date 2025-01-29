package valkeydempotency

import (
	"fmt"
	"log"
	"testing"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/valkey-io/valkey-go"
	"go.uber.org/goleak"
)

var testValkeyPortHost string

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not construct pool: %s", err)
	}

	// uses pool to try to connect to Docker
	err = pool.Client.Ping()
	if err != nil {
		log.Fatalf("Could not connect to Docker: %s", err)
	}

	// pulls an image, creates a container based on it and runs it
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "valkey/valkey",
		Tag:        "8",
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		log.Fatalf("Could not start resource: %s", err)
	}

	resource.Expire(240) // Tell docker to hard kill the container within 4mn

	testValkeyPortHost = resource.GetHostPort("6379/tcp")

	if err := pool.Retry(func() error {
		client, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{testValkeyPortHost}})
		if err != nil {
			return fmt.Errorf("could not connect to valkey: %w", err)
		}

		client.Close()

		return nil
	}); err != nil {
		log.Fatalf("Could not connect to valkey container: %s", err)
	}

	goleak.VerifyTestMain(m,
		goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).roundTrip"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
	)

	// You can't defer this because os.Exit doesn't care for defer
	if err := pool.Purge(resource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}
}
