package pgtest_test

import "github.com/induzo/gocom/database/pgtest"

func ExampleNew() { //nolint: testableexamples // dockertest no output
	dockertest := pgtest.New()

	// repo := NewRepo(dockertest.ConnPool)
	// do something...

	dockertest.Purge()
}
