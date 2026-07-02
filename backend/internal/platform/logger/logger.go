// Package logger provides a single, structured (slog) logger for the whole
// service. Every log line is JSON in production so it can be shipped to any
// log aggregator, and carries a request/trace id when available so a single
// request can be followed across every log line and audit entry it produced.
package logger

import (
	"context"
	"log/slog"
	"os"
)

type ctxKey struct{}

var base *slog.Logger

// New builds the process-wide logger. Call once at startup.
func New(level string, format string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}

	var handler slog.Handler
	if format == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	l := slog.New(handler)
	base = l
	slog.SetDefault(l)
	return l
}

// WithContext attaches a logger (already carrying request-scoped fields such
// as request_id/trace_id/actor) to a context.
func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext returns the request-scoped logger if present, otherwise the
// process-wide base logger.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	if base != nil {
		return base
	}
	return slog.Default()
}
