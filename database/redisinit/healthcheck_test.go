package redisinit

import (
	"context"
	"flag"
	"log"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/goleak"
)

var endpoint string //nolint:gochecknoglobals // test code

func TestMain(m *testing.M) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "redis:latest",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections"),
	}

	redisC, errP := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		},
	)
	if errP != nil {
		log.Fatalf("Could not run container: %s", errP)
	}

	defer func() {
		if err := redisC.Terminate(ctx); err != nil {
			log.Fatalf("Could not terminate container: %s", err)
		}
	}()

	var err error

	endpoint, err = redisC.Endpoint(ctx, "")
	if err != nil {
		log.Fatalf("Could not retrieve the container endpoint: %s", err)
	}

	leak := flag.Bool("leak", false, "use leak detector")
	flag.Parse()

	if *leak {
		goleak.VerifyTestMain(m, goleak.IgnoreAnyFunction("github.com/testcontainers/testcontainers-go.(*Reaper).Connect.func1"))

		return
	}

	os.Exit(m.Run())
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
		Addr: endpoint,
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
