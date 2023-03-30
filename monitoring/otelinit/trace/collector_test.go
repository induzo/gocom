package trace

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
)

func TestWithGRPCTraceExporter(t *testing.T) { //nolint: paralleltest // tracer uses global vars
	tests := []struct {
		name       string
		withOption []otlptracegrpc.Option
		wantErr    bool
	}{
		{
			name:       "no option",
			withOption: []otlptracegrpc.Option{},
		},
		{
			name: "with insecure",
			withOption: []otlptracegrpc.Option{
				otlptracegrpc.WithInsecure(),
			},
		},
		{
			name: "with collector",
			withOption: []otlptracegrpc.Option{
				otlptracegrpc.WithEndpoint("dwd"),
			},
		},
		{
			name: "with compressor",
			withOption: []otlptracegrpc.Option{
				otlptracegrpc.WithCompressor("gzip"),
			},
		},
		{
			name: "with headers",
			withOption: []otlptracegrpc.Option{
				otlptracegrpc.WithHeaders(map[string]string{"api-key": "123"}),
			},
		},
	}

	for _, tt := range tests { //nolint: paralleltest // tracer uses global vars
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			p := &provider{}

			if err := WithGRPCTraceExporter(ctx, tt.withOption...)(p); err != nil {
				t.Errorf("WithGRPCTraceExporter() expected error %t got %v", tt.wantErr, err)
			}

			if !tt.wantErr {
				sd, err := p.init(ctx)
				if err != nil {
					t.Errorf("WithGRPCTraceExporter() expected to init but got %v", err)
				}

				_ = sd()
			}
		})
	}
}
