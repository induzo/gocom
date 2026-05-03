package slog

import (
	"context"
	"log/slog"
	"reflect"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/tracelog"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// recordingHandler captures slog.Records so tests can assert on Level,
// Message, and emitted attributes.
type recordingHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordingHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.records = append(h.records, r.Clone())

	return nil
}

func (h *recordingHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(_ string) slog.Handler      { return h }

func (h *recordingHandler) snapshot() []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()

	out := make([]slog.Record, len(h.records))
	copy(out, h.records)

	return out
}

func TestNewLogger(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	want := &Logger{logger: logger}

	if got := NewLogger(logger); !reflect.DeepEqual(got, want) {
		t.Errorf("NewLogger() = %v, want %v", got, want)
	}
}

// BenchmarkNewLogger benchmarks the NewLogger function.
func BenchmarkNewLogger(b *testing.B) {
	logger := slog.New(slog.DiscardHandler)

	for b.Loop() {
		_ = NewLogger(logger)
	}
}

func TestLogger_Log(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		level       tracelog.LogLevel
		msg         string
		data        map[string]any
		wantLevel   slog.Level
		wantExtra   bool // PGX_LOG_LEVEL marker expected
		wantRecords int
	}{
		{
			name:        "Trace records at Debug with PGX_LOG_LEVEL marker",
			level:       tracelog.LogLevelTrace,
			msg:         "Trace message",
			data:        map[string]any{"a": "1", "b": "2"},
			wantLevel:   slog.LevelDebug,
			wantExtra:   true,
			wantRecords: 1,
		},
		{
			name:        "Debug records at Debug",
			level:       tracelog.LogLevelDebug,
			msg:         "Debug message",
			data:        map[string]any{"key": "value"},
			wantLevel:   slog.LevelDebug,
			wantRecords: 1,
		},
		{
			name:        "Info records at Info",
			level:       tracelog.LogLevelInfo,
			msg:         "Info message",
			data:        map[string]any{"key": "value"},
			wantLevel:   slog.LevelInfo,
			wantRecords: 1,
		},
		{
			name:        "Warn records at Warn",
			level:       tracelog.LogLevelWarn,
			msg:         "Warn message",
			data:        map[string]any{"key": "value"},
			wantLevel:   slog.LevelWarn,
			wantRecords: 1,
		},
		{
			name:        "Error records at Error",
			level:       tracelog.LogLevelError,
			msg:         "Error message",
			data:        map[string]any{"key": "value"},
			wantLevel:   slog.LevelError,
			wantRecords: 1,
		},
		{
			name:        "None emits no record",
			level:       tracelog.LogLevelNone,
			msg:         "None log level message",
			data:        map[string]any{"key": "value"},
			wantRecords: 0,
		},
		{
			name:        "Undefined records at Error with PGX_LOG_LEVEL marker",
			level:       tracelog.LogLevel(99),
			msg:         "Undefined log level message",
			data:        map[string]any{"key": "value"},
			wantLevel:   slog.LevelError,
			wantExtra:   true,
			wantRecords: 1,
		},
		{
			name:        "Nil data does not emit a stray attribute",
			level:       tracelog.LogLevelInfo,
			msg:         "info no data",
			data:        nil,
			wantLevel:   slog.LevelInfo,
			wantRecords: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := &recordingHandler{}
			pl := NewLogger(slog.New(h))

			pl.Log(context.Background(), tt.level, tt.msg, tt.data)

			records := h.snapshot()
			if len(records) != tt.wantRecords {
				t.Fatalf("records = %d, want %d", len(records), tt.wantRecords)
			}

			if tt.wantRecords == 0 {
				return
			}

			r := records[0]
			if r.Level != tt.wantLevel {
				t.Errorf("Level = %v, want %v", r.Level, tt.wantLevel)
			}

			if r.Message != tt.msg {
				t.Errorf("Message = %q, want %q", r.Message, tt.msg)
			}

			wantAttrs := len(tt.data)
			if tt.wantExtra {
				wantAttrs++
			}

			if r.NumAttrs() != wantAttrs {
				t.Errorf("NumAttrs = %d, want %d", r.NumAttrs(), wantAttrs)
			}

			if tt.wantExtra {
				found := false

				r.Attrs(func(a slog.Attr) bool {
					if a.Key == "PGX_LOG_LEVEL" {
						found = true
						return false
					}

					return true
				})

				if !found {
					t.Errorf("expected PGX_LOG_LEVEL attribute on record")
				}
			} else {
				r.Attrs(func(a slog.Attr) bool {
					if a.Key == "" || a.Key == "!BADKEY" {
						t.Errorf("unexpected stray attribute: %+v", a)
					}

					return true
				})
			}
		})
	}
}

// TestLogger_Log_DeterministicOrder asserts that for a given input, the
// emitted attribute order is stable (sorted by key).
func TestLogger_Log_DeterministicOrder(t *testing.T) {
	t.Parallel()

	h := &recordingHandler{}
	pl := NewLogger(slog.New(h))
	data := map[string]any{"c": 3, "a": 1, "b": 2}

	pl.Log(context.Background(), tracelog.LogLevelInfo, "msg", data)

	records := h.snapshot()
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}

	var keys []string

	records[0].Attrs(func(a slog.Attr) bool {
		keys = append(keys, a.Key)
		return true
	})

	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("attr keys = %v, want %v", keys, want)
	}
}

// BenchmarkLogger_Log benchmarks the Log method of the Logger.
func BenchmarkLogger_Log(b *testing.B) {
	logger := slog.New(slog.DiscardHandler)
	pl := NewLogger(logger)
	ctx := context.Background()
	level := tracelog.LogLevelInfo
	msg := "Benchmark message"
	data := map[string]any{"key": "value"}

	for b.Loop() {
		pl.Log(ctx, level, msg, data)
	}
}
