package redisinit

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/redis/go-redis/v9"
	"go.uber.org/goleak"
)

var endpoint string

func TestMain(m *testing.M) {
	// uses a sensible default on windows (tcp/http) and linux/osx (socket)
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not construct pool: %s", err)
	}

	// uses pool to try to connect to Docker
	errP := pool.Client.Ping()
	if errP != nil {
		log.Fatalf("Could not connect to Docker: %s", errP)
	}

	// pulls an image, creates a container based on it and runs it
	resource, errP := pool.RunWithOptions(
		&dockertest.RunOptions{
			Repository: "redis",
			Tag:        "7-alpine",
		}, func(config *docker.HostConfig) {
			// set AutoRemove to true so that stopped container goes away by itself
			config.AutoRemove = true
			config.RestartPolicy = docker.RestartPolicy{
				Name: "no",
			}
		})
	if errP != nil {
		log.Fatalf("Could not start resource: %s", errP)
	}

	endpoint = resource.GetHostPort("6379/tcp")

	if err := pool.Retry(func() error {
		if errNC := redis.NewClient(&redis.Options{
			Addr: endpoint,
		}).Ping(context.Background()).Err(); errNC != nil {
			return errNC
		}

		return nil
	}); err != nil {
		log.Fatalf("Could not connect to redis container: %s", err)
	}

	resource.Expire(60) // Tell docker to hard kill the container in 60 seconds

	goleak.VerifyTestMain(m,
		goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).roundTrip"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
	)

	code := m.Run()

	// You can't defer this because os.Exit doesn't care for defer
	if err := pool.Purge(resource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}

	os.Exit(code)
}

func TestClientHealthCheck(t *testing.T) {
	t.Parallel()

	type args struct {
		opt *redis.Options
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "happy",
			args: args{
				opt: &redis.Options{
					Addr: endpoint,
				},
			},
			wantErr: false,
		},
		{
			name: "error",
			args: args{
				opt: &redis.Options{
					Addr: "localhost:123",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cli := redis.NewClient(tt.args.opt)
			defer cli.Close()

			healthCheck := ClientHealthCheck(cli)
			if err := healthCheck(context.Background()); (err != nil) != tt.wantErr {
				t.Errorf("unexpected error in health check: %s", err)
			}
		})
	}
}

func BenchmarkXxx(b *testing.B) {
	cli := redis.NewClient(&redis.Options{
		Addr: endpoint,
	})
	defer cli.Close()

	healthCheck := ClientHealthCheck(cli)

	for b.Loop() {
		if err := healthCheck(context.Background()); err != nil {
			b.Errorf("unexpected error in health check: %s", err)
		}
	}
}
