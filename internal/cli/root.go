// Package cli wires up the cobra command tree and the viper-managed
// configuration. It is the only place where CLI surface area and
// application-level configuration meet.
//
// Core domain logic lives in internal/app and pkg/capture; this package
// is intentionally thin: parse flags, hydrate config, hand the user
// intent to the application.
package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/andrewhowdencom/babysafe/internal/config"
)

// loggerKey is the unexported context key under which the
// command-scoped logger is stored. See LoggerFromContext.
type loggerKey struct{}

// Execute is the entry point invoked by main.go. It builds the root
// command, attaches cobra's standard error and os.Exit behavior, and
// returns any error from command execution so that main can decide how
// to surface it.
func Execute() error {
	root, err := newRootCmd()
	if err != nil {
		return fmt.Errorf("build root command: %w", err)
	}

	if err := root.ExecuteContext(withRootLogger(root)); err != nil {
		return err
	}
	return nil
}

// withRootLogger returns a fresh context that carries the root
// command's logger. Subcommands inherit the same logger via
// WithLogger / LoggerFromContext.
func withRootLogger(root *cobra.Command) context.Context {
	logger := slog.Default().With("command", root.Name())
	return WithLogger(context.Background(), logger)
}

// WithLogger attaches a logger to ctx so that downstream code can pull
// it out with LoggerFromContext. Intentionally variadic-style (single
// function pair) so callers can pipe it through context easily.
func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, loggerKey{}, l)
}

// LoggerFromContext returns the logger attached to ctx, falling back
// to slog.Default() if none is set.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}
	if l, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}

// newRootCmd assembles the root command and registers subcommands.
// Splitting construction from execution makes the command tree testable
// in isolation.
func newRootCmd() (*cobra.Command, error) {
	cfg := config.NewDefault()

	cmd := &cobra.Command{
		Use:   "babysafe",
		Short: "Capture every Linux input device so that small humans can play safely.",
		Long: strings.TrimSpace(`babysafe grabs all available input devices on Linux so that a
baby (or any other small human) can sit at the keyboard without
accidentally triggering actions on the host.

The application is intentionally minimal: start it, watch a log line
that confirms every input device is now captured, and stop it when
play time is over.`),
		SilenceUsage:  true, // Don't print usage on runtime errors.
		SilenceErrors: true, // We render errors in main() ourselves.
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return bootstrap(cmd, cfg)
		},
	}

	cmd.PersistentFlags().String("log-level", "info", "Log level (debug, info, warn, error).")
	if err := viper.BindPFlag("log.level", cmd.PersistentFlags().Lookup("log-level")); err != nil {
		return nil, fmt.Errorf("bind log-level flag: %w", err)
	}

	cmd.PersistentFlags().String("config", "", "Path to a YAML configuration file (default: ./babysafe.yaml).")
	if err := viper.BindPFlag("config.path", cmd.PersistentFlags().Lookup("config")); err != nil {
		return nil, fmt.Errorf("bind config flag: %w", err)
	}

	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newRunCmd(cfg))

	return cmd, nil
}

// bootstrap is the single place where viper -> config hydration and
// logger setup happens. Returning an error here surfaces as a clean
// exit code rather than a panic.
func bootstrap(cmd *cobra.Command, cfg *config.Config) error {
	v := viper.New()

	v.SetEnvPrefix("BABYSAFE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	for k, val := range cfg.All() {
		v.SetDefault(k, val)
	}

	if path := v.GetString("config.path"); path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return fmt.Errorf("read config %q: %w", path, err)
			}
		}
	}

	if err := cfg.Hydrate(v); err != nil {
		return fmt.Errorf("hydrate config: %w", err)
	}

	logger, err := newLogger(cfg.Log.Level)
	if err != nil {
		return fmt.Errorf("configure logger: %w", err)
	}

	slog.SetDefault(logger)
	// Re-bind the root logger so subcommands see the freshly built one.
	cmd.SetContext(WithLogger(cmd.Context(), logger.With("command", cmd.Name())))
	return nil
}

// newLogger maps the configured string level to a slog.Level and wraps
// it in a TextHandler writing to stderr.
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
