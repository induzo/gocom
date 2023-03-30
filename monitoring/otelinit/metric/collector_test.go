package metric

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
)

func TestWithGRPCMetricExporter(t *testing.T) { //nolint: paralleltest // meter uses global vars
	tests := []struct {
		name       string
		withOption []otlpmetricgrpc.Option
		wantErr    bool
	}{
		{
			name:       "no option",
			withOption: []otlpmetricgrpc.Option{},
		},
		{
			name: "with insecure",
			withOption: []otlpmetricgrpc.Option{
				otlpmetricgrpc.WithInsecure(),
			},
		},
		{
			name: "with collector",
			withOption: []otlpmetricgrpc.Option{
				otlpmetricgrpc.WithEndpoint("dwd"),
			},
		},
		{
			name: "with compressor",
			withOption: []otlpmetricgrpc.Option{
				otlpmetricgrpc.WithCompressor("gzip"),
			},
		},
		{
			name: "with headers",
			withOption: []otlpmetricgrpc.Option{
				otlpmetricgrpc.WithHeaders(map[string]string{"api-key": "123"}),
			},
		},
	}

	for _, tt := range tests { //nolint: paralleltest // tracer uses global vars
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			p := &provider{}

			tt.withOption = append(tt.withOption,
				otlpmetricgrpc.WithInsecure(),
				otlpmetricgrpc.WithEndpoint("localhost:"+testPort),
			)

			if err := WithGRPCMetricExporter(ctx, tt.withOption...)(p); err != nil {
				t.Errorf("WithGRPCMetricExporter() expected error %t got %v", tt.wantErr, err)
			}

			if !tt.wantErr {
				sd, err := p.init(ctx)
				if err != nil {
					t.Errorf("WithGRPCMetricExporter() expected to init but got %v", err)
				}

				_ = sd()
			}
		})
	}
}
