// Package cli wires up the cobra command tree. It is the only place
// where CLI surface area meets the underlying capture primitives.
//
// babysafe is deliberately a CLI-only tool: there is no YAML
// configuration file, no environment-variable knobs (other than the
// standard POSIX signal handling). Everything is expressed as cobra
// flags so the help text is always discoverable.
package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

// Execute is the entry point invoked by main.go. It builds the root
// command, hooks SIGINT/SIGTERM into a cancellable context, and
// returns any error from command execution.
func Execute() error {
	root, err := newRootCmd()
	if err != nil {
		return fmt.Errorf("build root command: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return root.ExecuteContext(ctx)
}

// newRootCmd assembles the root command and registers subcommands.
// Splitting construction from execution makes the command tree
// testable in isolation.
func newRootCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "babysafe",
		Short: "Capture Linux input devices so that small humans can play safely.",
		Long: strings.TrimSpace(`babysafe grabs Linux input devices on demand so that a
baby (or any other small human) can sit at the keyboard without
accidentally triggering actions on the host.

Typical usage:

  sudo babysafe list
  sudo babysafe grab --match type=keyboard
  sudo babysafe grab --match name=Logitech
  sudo babysafe grab --match path=/dev/input/by-id/usb-Logitech_*-event-kbd

Devices are released automatically on SIGINT / SIGTERM.`),
		SilenceUsage:  true, // Don't print usage on runtime errors.
		SilenceErrors: true, // We render errors in main() ourselves.
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return bootstrapLogger(cmd)
		},
	}

	cmd.PersistentFlags().String("log-level", "info", "Log level (debug, info, warn, error).")

	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newGrabCmd())

	return cmd, nil
}

// bootstrapLogger configures slog from --log-level and stores the
// resulting logger on the command context so subcommands can pull it
// out via LoggerFromContext.
func bootstrapLogger(cmd *cobra.Command) error {
	level, err := cmd.Flags().GetString("log-level")
	if err != nil {
		return fmt.Errorf("read --log-level: %w", err)
	}

	logger, err := newLogger(level)
	if err != nil {
		return err
	}
	slog.SetDefault(logger)

	cmd.SetContext(WithLogger(cmd.Context(), logger.With("command", cmd.Name())))
	return nil
}

// newLogger maps a string level to slog.Level and returns a
// TextHandler writing to stderr.
func newLogger(level string) (*slog.Logger, error) {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "info", "":
		lvl = slog.LevelInfo
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		return nil, fmt.Errorf("unknown log level %q (want debug, info, warn, error)", level)
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})), nil
}
