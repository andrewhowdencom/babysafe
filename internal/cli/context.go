package cli

import (
	"context"
	"log/slog"
)

// loggerKey is the unexported context key under which the
// command-scoped logger is stored. See WithLogger / LoggerFromContext.
type loggerKey struct{}

// WithLogger attaches a logger to ctx so downstream code can pull it
// out with LoggerFromContext. Safe with a nil ctx.
func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, loggerKey{}, l)
}

// LoggerFromContext returns the logger attached to ctx, falling back
// to slog.Default() if none is set. Safe with a nil ctx.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}
	if l, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}
