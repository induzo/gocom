package redistest

import (
	"flag"
	"log"
	"os"
	"reflect"
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	leak := flag.Bool("leak", false, "use leak detector")
	flag.Parse()

	code := m.Run()

	if *leak {
		if code == 0 {
			if err := goleak.Find(); err != nil {
				log.Fatalf("goleak: Errors on successful test run: %v\n", err)

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

			resource := New()
			resource.Purge()

			if reflect.TypeOf(resource) != reflect.TypeOf(tt.want) {
				t.Errorf("returned %v is not want %v", resource, tt.want)
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
