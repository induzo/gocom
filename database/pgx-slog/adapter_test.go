package slog

import (
	"context"
	"flag"
	"io"
	"log/slog"
	"os"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/tracelog"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	leak := flag.Bool("leak", false, "use leak detector")
	flag.Parse()

	if *leak {
		goleak.VerifyTestMain(m)

		return
	}

	os.Exit(m.Run())
}

func TestNewLogger(t *testing.T) {
	t.Parallel()

	textHandler := slog.NewTextHandler(io.Discard, nil)
	logger := slog.New(textHandler)
	want := &Logger{logger: logger}

	if got := NewLogger(logger); !reflect.DeepEqual(got, want) {
		t.Errorf("NewLogger() = %v, want %v", got, want)
	}
}

// BenchmarkNewLogger benchmarks the NewLogger function.
func BenchmarkNewLogger(b *testing.B) {
	textHandler := slog.NewTextHandler(io.Discard, nil)
	logger := slog.New(textHandler)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = NewLogger(logger)
	}
}

func TestLogger_Log(t *testing.T) {
	t.Parallel()

	textHandler := slog.NewTextHandler(io.Discard, nil)
	logger := slog.New(textHandler)
	pl := NewLogger(logger)

	type args struct {
		level tracelog.LogLevel
		msg   string
		data  map[string]interface{}
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "LogLevelTrace",
			args: args{
				level: tracelog.LogLevelTrace,
				msg:   "Trace message",
				data:  map[string]interface{}{"key": "value"},
			},
		},
		{
			name: "LogLevelDebug",
			args: args{
				level: tracelog.LogLevelDebug,
				msg:   "Debug message",
				data:  map[string]interface{}{"key": "value"},
			},
		},
		{
			name: "LogLevelInfo",
			args: args{
				level: tracelog.LogLevelInfo,
				msg:   "Info message",
				data:  map[string]interface{}{"key": "value"},
			},
		},
		{
			name: "LogLevelWarn",
			args: args{
				level: tracelog.LogLevelWarn,
				msg:   "Warn message",
				data:  map[string]interface{}{"key": "value"},
			},
		},
		{
			name: "LogLevelError",
			args: args{
				level: tracelog.LogLevelError,
				msg:   "Error message",
				data:  map[string]interface{}{"key": "value"},
			},
		},
		{
			name: "LogLevelNone",
			args: args{
				level: tracelog.LogLevelNone,
				msg:   "None log level message",
				data:  map[string]interface{}{"key": "value"},
			},
		},
		{
			name: "LogLevelUndefined",
			args: args{
				level: tracelog.LogLevel(99),
				msg:   "Undefined log level message",
				data:  map[string]interface{}{"key": "value"},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pl.Log(context.Background(), tt.args.level, tt.args.msg, tt.args.data)
		})
	}
}

// BenchmarkLogger_Log benchmarks the Log method of the Logger.
func BenchmarkLogger_Log(b *testing.B) {
	textHandler := slog.NewTextHandler(io.Discard, nil)
	logger := slog.New(textHandler)

	pl := NewLogger(logger)
	ctx := context.Background()
	level := tracelog.LogLevelInfo
	msg := "Benchmark message"
	data := map[string]interface{}{"key": "value"}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pl.Log(ctx, level, msg, data)
	}
}
