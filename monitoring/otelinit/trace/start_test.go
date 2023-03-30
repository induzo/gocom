package trace

import (
	"context"
	"testing"
)

func TestStart(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		withAPIKey bool
	}{
		{
			name:       "happy path, no api key",
			withAPIKey: false,
		},
		{
			name:       "happy path, with api key",
			withAPIKey: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			apiKey := ""
			if tt.withAPIKey {
				apiKey = "123"
			}

			sd, err := Start(ctx, tt.name, "127.0.0.1", 4317, apiKey, false)
			if err != nil {
				t.Errorf("error starting otel: %v", err)

				return
			}

			if sd != nil {
				if err := sd(ctx); err != nil {
					t.Errorf("error shutdown otel: %v", err)

					return
				}
			}
		})
	}
}
