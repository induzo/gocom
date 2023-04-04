package redistest

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/redis/go-redis/v9"
)

const (
	poolMaxWait    = 120 * time.Second
	resourceExpire = 180
)

type DockertestWrapper struct {
	DockertestPool     *dockertest.Pool
	DockertestResource *dockertest.Resource
	RedisAddr          string
}

func New() *DockertestWrapper {
	var (
		pool     *dockertest.Pool
		resource *dockertest.Resource
		err      error
	)

	pool, err = dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	pool.MaxWait = poolMaxWait

	resource, err = pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "redis",
		Tag:        "7",
		Env: []string{
			"REDIS_PASSWORD=redis",
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		log.Fatalf("Could not start resource: %s", err)
	}

	if err = resource.Expire(resourceExpire); err != nil {
		log.Fatalf("Could not start resource: %s", err)
	}

	if errP := pool.Retry(func() error {
		redisCli := redis.NewClient(&redis.Options{
			Addr:            getHostPort(resource, "6379/tcp"),
			ConnMaxIdleTime: time.Second,
		})
		defer redisCli.Close()

		errS := redisCli.Set(
			context.Background(),
			"test_connection",
			"test_connection",
			time.Second).Err()
		if errS != nil {
			return fmt.Errorf("%w", err)
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
		RedisAddr:          getHostPort(resource, "6379/tcp"),
	}
}

func (dw *DockertestWrapper) Purge() {
	if err := dw.DockertestPool.Purge(dw.DockertestResource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}
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
