<!-- Code generated by gomarkdoc. DO NOT EDIT -->

# redisinit

```go
import "github.com/induzo/gocom/database/redisinit"
```

This package allows you to init a redis client via go\-redis.

## Index

- [func ClientHealthCheck\[T RedisClient\[U\], U RedisError\]\(cli T\) func\(ctx context.Context\) error](<#ClientHealthCheck>)
- [type RedisClient](<#RedisClient>)
- [type RedisError](<#RedisError>)


<a name="ClientHealthCheck"></a>
## func [ClientHealthCheck](<https://github.com/induzo/gocom/blob/main/database/redisinit/healthcheck.go#L20>)

```go
func ClientHealthCheck[T RedisClient[U], U RedisError](cli T) func(ctx context.Context) error
```

ClientHealthCheck returns a health check function for redis.Client that can be used in health endpoint.

<details><summary>Example</summary>
<p>

Using standard net/http package. We can also simply pass healthCheck as a CheckFn in gocom/transport/http/health/v2.

```go
ctx := context.Background()

cli := redis.NewClient(&redis.Options{
	Addr: "localhost:6379",
})

healthCheck := redisinit.ClientHealthCheck(cli)

mux := http.NewServeMux()

mux.HandleFunc("/sys/health", func(rw http.ResponseWriter, _ *http.Request) {
	if err := healthCheck(ctx); err != nil {
		rw.WriteHeader(http.StatusServiceUnavailable)
	}
})

req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "/sys/health", nil)
nr := httptest.NewRecorder()

mux.ServeHTTP(nr, req)

rr := nr.Result()
defer rr.Body.Close()

fmt.Println(rr.StatusCode)
```

</p>
</details>

<a name="RedisClient"></a>
## type [RedisClient](<https://github.com/induzo/gocom/blob/main/database/redisinit/healthcheck.go#L9-L12>)



```go
type RedisClient[T RedisError] interface {
    *redis.Client
    Ping(context.Context) T
}
```

<a name="RedisError"></a>
## type [RedisError](<https://github.com/induzo/gocom/blob/main/database/redisinit/healthcheck.go#L14-L17>)



```go
type RedisError interface {
    *redis.StatusCmd
    Err() error
}
```

Generated by [gomarkdoc](<https://github.com/princjef/gomarkdoc>)
