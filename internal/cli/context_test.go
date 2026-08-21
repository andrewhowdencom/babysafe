package cli

import (
	"context"
	"log/slog"
	"testing"
)

func TestLoggerFromContext(t *testing.T) {
	// LoggerFromContext tolerates a nil context — it falls back to
	// slog.Default(). This is a deliberate API choice so that callers
	// don't have to special-case the "no parent context" path.
	//
	//nolint:staticcheck // SA1012: nil context is intentional here.
	if got := LoggerFromContext(nil); got == nil {
		t.Errorf("LoggerFromContext(nil) returned nil")
	}

	custom := slog.New(slog.NewTextHandler(testWriter{}, nil)).With("k", "v")
	ctx := WithLogger(context.Background(), custom)
	if got := LoggerFromContext(ctx); got != custom {
		t.Errorf("LoggerFromContext returned %v, want the attached logger", got)
	}

	// Nil custom logger should fall back to default.
	if got := LoggerFromContext(WithLogger(context.Background(), nil)); got == nil {
		t.Errorf("LoggerFromContext with nil attached logger should fall back to default")
	}
}

// testWriter is a no-op io.Writer so we don't have to import io in
// every test file.
type testWriter struct{}

func (testWriter) Write(p []byte) (int, error) { return len(p), nil }
