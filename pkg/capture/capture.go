// Package capture holds the Linux input-device grab primitives.
//
// The package is intentionally placed under pkg/ (not internal/) so
// that other Go programs can import it as a library: any tool that
// wants to "vacuum up" the local input devices can depend directly on
// pkg/capture without pulling in the babysafe CLI.
//
// # Linux only
//
// The package depends on the Linux EVIOCGRAB ioctl, exposed by
// github.com/holoplot/go-evdev. Behavior on non-Linux operating
// systems is undefined and unsupported.
package capture

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	evdev "github.com/holoplot/go-evdev"
)

// DefaultDeviceGlob is the path glob used when none is configured.
const DefaultDeviceGlob = "/dev/input/event*"

// Options configures a capture Session.
type Options struct {
	// DeviceGlobs are shell-style globs resolved against the filesystem.
	DeviceGlobs []string

	// Excludes is a list of substrings. Any device path containing one
	// of these substrings is skipped. Useful for ignoring a touchscreen
	// or a particular keyboard.
	Excludes []string

	// ReleaseOnExit controls whether grabbed devices are released when
	// the session is closed.
	ReleaseOnExit bool
}

// Session represents an active grab over one or more input devices.
type Session interface {
	// DeviceCount returns the number of devices currently held.
	DeviceCount() int

	// Run blocks until ctx is cancelled or a fatal error occurs.
	Run(ctx context.Context) error

	// Close releases any held devices. Idempotent.
	Close(ctx context.Context) error
}

// NewSession validates options, enumerates matching devices, and
// returns a Session ready to be Run. If grabbing any device fails the
// session is closed before the error is returned so the caller isn't
// left holding a partial grab.
func NewSession(ctx context.Context, opts Options) (Session, error) {
	if len(opts.DeviceGlobs) == 0 {
		opts.DeviceGlobs = []string{DefaultDeviceGlob}
	}

	paths, err := resolveDevices(opts.DeviceGlobs, opts.Excludes)
	if err != nil {
		return nil, fmt.Errorf("resolve devices: %w", err)
	}

	logger := slog.Default().With("component", "capture")
	s := &session{
		opts:   opts,
		paths:  paths,
		grabs:  make(map[string]*evdev.InputDevice),
		logger: logger,
	}

	if err := s.grabAll(ctx); err != nil {
		// Best-effort release so the caller isn't left holding devices.
		// Strip cancellation from the parent so a cancelled ctx can't
		// prevent us from releasing the grabs we already made.
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = s.Close(releaseCtx)
		return nil, err
	}

	return s, nil
}

// session is the concrete Session implementation.
type session struct {
	opts   Options
	paths  []string
	mu     sync.Mutex
	grabs  map[string]*evdev.InputDevice
	logger *slog.Logger
}

// DeviceCount returns the number of devices currently held.
func (s *session) DeviceCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.grabs)
}

// Run parks the goroutine until ctx is done. There is no event
// processing in the skeleton — once devices are grabbed, they're
// swallowed by the kernel until release.
func (s *session) Run(ctx context.Context) error {
	<-ctx.Done()
	if err := ctx.Err(); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("session context: %w", err)
	}
	return nil
}

// Close releases every grabbed device. Safe to call repeatedly.
func (s *session) Close(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var firstErr error
	for path, dev := range s.grabs {
		if err := dev.Ungrab(); err != nil {
			s.logger.Warn("ungrab device failed", "path", path, "err", err)
			if firstErr == nil {
				firstErr = fmt.Errorf("ungrab %s: %w", path, err)
			}
			// Still try to close the underlying fd.
		}
		if err := dev.Close(); err != nil {
			s.logger.Warn("close device failed", "path", path, "err", err)
			if firstErr == nil {
				firstErr = fmt.Errorf("close %s: %w", path, err)
			}
		}
		delete(s.grabs, path)
	}
	return firstErr
}

// grabAll opens every matched device and issues an EVIOCGRAB.
func (s *session) grabAll(_ context.Context) error {
	for _, path := range s.paths {
		dev, err := evdev.Open(path)
		if err != nil {
			return fmt.Errorf("open %s: %w (root required)", path, err)
		}

		if err := dev.Grab(); err != nil {
			_ = dev.Close()
			return fmt.Errorf("grab %s: %w", path, err)
		}

		s.mu.Lock()
		s.grabs[path] = dev
		s.mu.Unlock()

		s.logger.Info("grabbed input device", "path", path)
	}
	return nil
}

// resolveDevices expands every glob, then filters out paths that
// contain any of the exclude substrings.
func resolveDevices(globs, excludes []string) ([]string, error) {
	var matches []string

	for _, g := range globs {
		expanded, err := filepath.Glob(g)
		if err != nil {
			return nil, fmt.Errorf("invalid glob %q: %w", g, err)
		}
		matches = append(matches, expanded...)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no input devices matched globs %v", globs)
	}

	filtered := matches[:0]
nextMatch:
	for _, m := range matches {
		for _, e := range excludes {
			if strings.Contains(m, e) {
				continue nextMatch
			}
		}
		filtered = append(filtered, m)
	}

	if len(filtered) == 0 {
		return nil, fmt.Errorf("every device was excluded by %v", excludes)
	}

	return filtered, nil
}
