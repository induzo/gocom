package redistest_test

import "github.com/induzo/gocom/database/redistest"

func ExampleNew() { //nolint: testableexamples // redistest no output
	dockertest := redistest.New()

	// repo := NewRepo(dockertest.RedisAddr)
	// do something...

	dockertest.Purge()
}
