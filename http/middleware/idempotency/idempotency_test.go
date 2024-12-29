package idempotency

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, goleak.IgnoreAnyFunction("github.com/testcontainers/testcontainers-go.(*Reaper).Connect.func1"))
}
