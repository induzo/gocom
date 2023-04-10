package redisinit

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/ory/dockertest/v3"
	"github.com/redis/go-redis/v9"
	"go.uber.org/goleak"
)

var testPort string //nolint:gochecknoglobals // redis dockertest info

func TestMain(m *testing.M) {
	var err error

	dockerPool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	resource, err := dockerPool.Run("redis", "7", nil)
	if err != nil {
		log.Fatalf("Could not start resource: %s", err)
	}

	var redisCli *redis.Client

	testPort = resource.GetPort("6379/tcp")

	if errP := dockerPool.Retry(func() error {
		redisCli = redis.NewClient(&redis.Options{
			Addr: fmt.Sprintf("localhost:%s", testPort),
		})

		return redisCli.Ping(context.Background()).Err()
	}); errP != nil {
		log.Fatalf("Could not connect to docker: %s", errP)
	}

	defer redisCli.Close()

	leak := flag.Bool("leak", false, "use leak detector")
	flag.Parse()

	code := m.Run()

	if *leak {
		if code == 0 {
			if err := goleak.Find(); err != nil {
				log.Fatalf("goleak: Errors on successful test run: %v\n", err) //nolint:revive // test code

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
					Addr: fmt.Sprintf("localhost:%s", testPort),
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
		tt := tt
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
		Addr: fmt.Sprintf("localhost:%s", testPort),
	})
	defer cli.Close()

	healthCheck := ClientHealthCheck(cli)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := healthCheck(context.Background()); err != nil {
			b.Errorf("unexpected error in health check: %s", err)
		}
	}
}
