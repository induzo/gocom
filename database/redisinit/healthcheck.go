package redisinit

import (
	"context"

	redis "github.com/redis/go-redis/v9"
)

type RedisClient[T RedisError] interface {
	*redis.Client
	Ping(context.Context) T
}

type RedisError interface {
	*redis.StatusCmd
	Err() error
}

// ClientHealthCheck returns a health check function for redis.Client that can be used in health endpoint.
func ClientHealthCheck[T RedisClient[U], U RedisError](cli T) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return cli.Ping(ctx).Err() //nolint:wrapcheck // health check fn
	}
}
