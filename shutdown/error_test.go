package shutdown

import (
	"errors"
	"testing"
)

func TestShutdownError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		err            error
		expectedString []string
	}{
		{
			name: "happy path",
			err: shutdownError(map[string]error{
				"test1": errors.New("dummy error 1"),
				"test2": errors.New("dummy error 2"),
			}),
			expectedString: []string{
				"test1 err: dummy error 1, test2 err: dummy error 2",
				"test2 err: dummy error 2, test1 err: dummy error 1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// everytime we do Error(), might be different order
			gotErr := tt.err.Error()

			found := false

			for _, expectedString := range tt.expectedString {
				if expectedString == gotErr {
					found = true
				}
			}

			if !found {
				t.Errorf("got error string = %v, want error string in %v", gotErr, tt.expectedString)
			}
		})
	}
}
