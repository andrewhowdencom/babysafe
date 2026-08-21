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
	"os"
	"sync"
	"time"

	evdev "github.com/holoplot/go-evdev"
)

// Session represents an active grab over one or more input devices.
type Session interface {
	// Devices returns the paths of every device currently held.
	Devices() []string

	// Run blocks until ctx is cancelled or a fatal error occurs.
	Run(ctx context.Context) error

	// Close releases any held devices. Idempotent.
	Close(ctx context.Context) error
}

// Options configures a capture Session. Matchers are AND-ed: a device
// must satisfy every Matcher to be grabbed. Excludes are OR-ed: a
// device that matches any Exclude is skipped.
type Options struct {
	Matchers []Matcher
	Excludes []Matcher
	Logger   *slog.Logger // Optional; defaults to slog.Default().
}

// NewSession lists /dev/input/event* devices, applies matchers and
// excludes, and grabs every surviving device. If any grab fails, all
// previously-opened grabs are released before the error is returned.
func NewSession(ctx context.Context, opts Options) (Session, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With("component", "capture")

	devs, err := ListDevices(ctx, logger)
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}

	s := &session{logger: logger, grabs: make(map[string]*evdev.InputDevice)}
	for _, dev := range devs {
		if !matchAll(dev, opts.Matchers) {
			continue
		}
		if matchAny(dev, opts.Excludes) {
			continue
		}

		if err := s.openAndGrab(dev.Path); err != nil {
			// Best-effort release so we never leave a partial grab.
			releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			_ = s.Close(releaseCtx)
			cancel()
			return nil, err
		}
	}

	if len(s.grabs) == 0 {
		return nil, errors.New("no devices matched the requested filters")
	}

	return s, nil
}

// session is the concrete Session implementation.
type session struct {
	mu     sync.Mutex
	grabs  map[string]*evdev.InputDevice
	logger *slog.Logger
}

// Devices returns the paths of every device currently held. The order
// is not stable; callers that care should sort the result.
func (s *session) Devices() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.grabs))
	for p := range s.grabs {
		out = append(out, p)
	}
	return out
}

// Run parks the goroutine until ctx is done. While parked, it actively
// drains events from each grabbed device so the kernel-side event
// queue does not grow unbounded, and so the firmware of certain
// keyboards (e.g., Logitech G512) does not see an unresponsive host —
// which would otherwise get the firmware's autorepeat state stuck and
// cause release events to be lost. Events are read and discarded;
// there is no event processing in the skeleton.
func (s *session) Run(ctx context.Context) error {
	s.mu.Lock()
	for _, dev := range s.grabs {
		s.startDrain(dev)
	}
	s.mu.Unlock()

	<-ctx.Done()
	if err := ctx.Err(); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("session context: %w", err)
	}
	return nil
}

// startDrain launches a goroutine that reads events from dev and
// discards them. The goroutine exits when ReadOne returns an error —
// typically because the device was closed by Close(). Any other error
// is logged at debug level so it can be diagnosed without making the
// happy path noisy.
func (s *session) startDrain(dev *evdev.InputDevice) {
	go func() {
		for {
			if _, err := dev.ReadOne(); err != nil {
				if !errors.Is(err, os.ErrClosed) {
					s.logger.Debug("event drain ended unexpectedly",
						"path", dev.Path(), "err", err)
				}
				return
			}
		}
	}()
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

// openAndGrab opens the file at path (which may be a /dev/input/by-id
// symlink) and issues EVIOCGRAB on the resulting file descriptor. The
// kernel resolves the symlink; the fd is bound to the underlying
// /dev/input/eventN, so Ungrab/Close always operate on the right
// device even if the symlink disappears mid-grab.
func (s *session) openAndGrab(path string) error {
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
	return nil
}

// matchAll returns true iff every matcher returns true (or there are
// no matchers).
func matchAll(d *Device, ms []Matcher) bool {
	for _, m := range ms {
		if !m(d) {
			return false
		}
	}
	return true
}

// matchAny returns true iff any matcher returns true.
func matchAny(d *Device, ms []Matcher) bool {
	for _, m := range ms {
		if m(d) {
			return true
		}
	}
	return false
}
