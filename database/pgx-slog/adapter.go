// Package slog provides a logger that writes to a go.uber.org/slog.Logger.
package slog

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/tracelog"
)

type Logger struct {
	logger *slog.Logger
}

func NewLogger(logger *slog.Logger) *Logger {
	return &Logger{logger: logger}
}

func (pl *Logger) Log(_ context.Context, level tracelog.LogLevel, msg string, data map[string]interface{}) {
	fields := make([]any, len(data)+1)
	idx := 0

	for k, v := range data {
		fields[idx] = slog.Any(k, v)
		idx++
	}

	switch level {
	case tracelog.LogLevelTrace:
		fields[idx] = slog.String("PGX_LOG_LEVEL", level.String())
		pl.logger.Debug(msg, fields...)
	case tracelog.LogLevelDebug:
		pl.logger.Debug(msg, fields...)
	case tracelog.LogLevelInfo:
		pl.logger.Info(msg, fields...)
	case tracelog.LogLevelWarn:
		pl.logger.Warn(msg, fields...)
	case tracelog.LogLevelError:
		pl.logger.Error(msg, fields...)
	case tracelog.LogLevelNone:
	default:
		fields[idx] = slog.String("PGX_LOG_LEVEL", level.String())
		pl.logger.Error(msg, fields...)
	}
}
