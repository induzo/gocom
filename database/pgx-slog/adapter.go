// Package slog adapts pgx's tracelog.Logger to the standard library log/slog.
package slog

import (
	"context"
	"log/slog"
	"slices"

	"github.com/jackc/pgx/v5/tracelog"
)

var _ tracelog.Logger = (*Logger)(nil)

// Logger forwards pgx tracelog records to a *slog.Logger.
type Logger struct {
	logger *slog.Logger
}

// NewLogger returns a Logger that forwards pgx tracelog records to logger.
func NewLogger(logger *slog.Logger) *Logger {
	return &Logger{logger: logger}
}

// Log implements tracelog.Logger by forwarding to the underlying *slog.Logger.
// LogLevelNone is honored as a no-op. Trace and undefined pgx levels are
// emitted at slog.LevelDebug and slog.LevelError respectively, with an extra
// PGX_LOG_LEVEL attribute identifying the original pgx level. Attributes are
// emitted in deterministic (sorted) key order.
func (pl *Logger) Log(
	ctx context.Context,
	level tracelog.LogLevel,
	msg string,
	data map[string]any,
) {
	if level == tracelog.LogLevelNone {
		return
	}

	attrs := buildAttrs(data)

	switch level {
	case tracelog.LogLevelTrace:
		attrs = append(attrs, slog.String("PGX_LOG_LEVEL", level.String()))
		pl.logger.DebugContext(ctx, msg, attrs...)
	case tracelog.LogLevelDebug:
		pl.logger.DebugContext(ctx, msg, attrs...)
	case tracelog.LogLevelInfo:
		pl.logger.InfoContext(ctx, msg, attrs...)
	case tracelog.LogLevelWarn:
		pl.logger.WarnContext(ctx, msg, attrs...)
	case tracelog.LogLevelError:
		pl.logger.ErrorContext(ctx, msg, attrs...)
	case tracelog.LogLevelNone:
		// already handled by the early return above.
	default:
		attrs = append(attrs, slog.String("PGX_LOG_LEVEL", level.String()))
		pl.logger.ErrorContext(ctx, msg, attrs...)
	}
}

func buildAttrs(data map[string]any) []any {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}

	slices.Sort(keys)

	attrs := make([]any, 0, len(keys)+1)
	for _, k := range keys {
		attrs = append(attrs, slog.Any(k, data[k]))
	}

	return attrs
}
