package pgtest

import "testing"

func TestConnPoolNotFoundError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		err            error
		expectedString string
	}{
		{
			name:           "happy path",
			err:            &ConnPoolNotFoundError{},
			expectedString: "connpool is nil",
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.err.Error() != tt.expectedString {
				t.Error("unexpected Error string")
			}
		})
	}
}
