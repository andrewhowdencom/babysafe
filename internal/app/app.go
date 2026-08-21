// Package app wires configuration and the input-capture primitives into
// a single struct that the CLI invokes. App is the seam between the
// CLI layer and the pkg/capture library; it should remain thin.
package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/andrewhowdencom/babysafe/internal/config"
	"github.com/andrewhowdencom/babysafe/pkg/capture"
)

// Options configures App construction. Use this when you want to
// override defaults at construction time (e.g., in tests).
type Options struct {
	ReleaseOnExit bool
}

// App is the concrete babysafe application. It owns the capture
// session and is responsible for releasing it on shutdown.
type App struct {
	cfg     *config.Config
	capture capture.Session
	logger  *slog.Logger
}

// New constructs an App. The capture session is opened lazily — the
// first call to Run grabs the devices — so that command-line help /
// version subcommands can run without root.
func New(ctx context.Context, cfg *config.Config, opts Options) (*App, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	// If the caller (CLI) didn't pass an explicit release-on-exit, defer
	// to whatever the config file said.
	if !opts.ReleaseOnExit {
		cfg.Capture.ReleaseOnExit = false
	}

	logger := slog.Default().With("component", "app")

	caps := capture.Options{
		DeviceGlobs:   cfg.Capture.DeviceGlobs,
		Excludes:      cfg.Capture.Excludes,
		ReleaseOnExit: cfg.Capture.ReleaseOnExit,
	}

	sess, err := capture.NewSession(ctx, caps)
	if err != nil {
		return nil, fmt.Errorf("open capture session: %w", err)
	}

	return &App{
		cfg:     cfg,
		capture: sess,
		logger:  logger,
	}, nil
}

// DeviceCount is the number of input devices currently grabbed.
func (a *App) DeviceCount() int { return a.capture.DeviceCount() }

// Run blocks until ctx is cancelled or a fatal error occurs. The
// capture session holds onto the grabbed devices for the lifetime of
// this call.
func (a *App) Run(ctx context.Context) error {
	a.logger.Info("babysafe: holding input devices",
		"devices", a.capture.DeviceCount(),
		"release_on_exit", a.cfg.Capture.ReleaseOnExit,
	)

	if err := a.capture.Run(ctx); err != nil {
		a.logger.Error("capture session failed", "err", err)
		return err
	}

	a.logger.Info("babysafe: released input devices cleanly")
	return nil
}

// Shutdown releases any held devices. Safe to call multiple times.
func (a *App) Shutdown(ctx context.Context) error {
	if err := a.capture.Close(ctx); err != nil {
		return fmt.Errorf("close capture session: %w", err)
	}
	return nil
}
