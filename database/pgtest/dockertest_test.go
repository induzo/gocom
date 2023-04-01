package pgtest

import (
	"flag"
	"log"
	"os"
	"reflect"
	"testing"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"go.uber.org/goleak"

	"github.com/induzo/gocom/database/pgtest/database"
)

func TestMain(m *testing.M) {
	leak := flag.Bool("leak", false, "use leak detector")
	flag.Parse()

	code := m.Run()

	if *leak {
		if code == 0 {
			if err := goleak.Find(); err != nil {
				log.Fatalf("goleak: Errors on successful test run: %v\n", err) //nolint: revive // test code

				code = 1
			}
		}
	}

	os.Exit(code)
}

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want *DockertestWrapper
	}{
		{
			name: "happy",
			want: &DockertestWrapper{},
		},
	}
	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dtWrapper := New()
			dtWrapper.Purge()

			if reflect.TypeOf(dtWrapper) != reflect.TypeOf(tt.want) {
				t.Errorf("returned %v is not want %v", dtWrapper, tt.want)
			}
		})
	}
}

func TestPrepareTestCaseDB(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want *DockertestWrapper
	}{
		{
			name: "happy",
			want: &DockertestWrapper{},
		},
	}
	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dtWrapper := New()
			pool, err := dtWrapper.PrepareTestCaseDB(database.TestMigrationFiles, "migrations")
			pool.Close()
			dtWrapper.Purge()

			if err != nil {
				t.Errorf("PrepareTestCaseDB got err: %v", err)
			}
		})
	}
}

func BenchmarkNew(b *testing.B) {
	for i := 0; i <= b.N; i++ {
		resource := New()
		resource.Purge()
	}
}
