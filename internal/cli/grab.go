package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	evdev "github.com/holoplot/go-evdev"
	"github.com/spf13/cobra"

	"github.com/andrewhowdencom/babysafe/internal/animation"
	"github.com/andrewhowdencom/babysafe/pkg/capture"
)

type grabRenderer interface {
	Submit(*evdev.InputEvent)
	Run(context.Context) error
	Close() error
}

type grabDependencies struct {
	newRenderer func(io.Writer) (grabRenderer, error)
	newSession  func(context.Context, capture.Options) (capture.Session, error)
}

func defaultGrabDependencies() grabDependencies {
	return grabDependencies{
		newRenderer: func(output io.Writer) (grabRenderer, error) {
			return animation.NewRenderer(output)
		},
		newSession: capture.NewSession,
	}
}

// newGrabCmd is the workhorse command: grab every input device that matches
// the supplied filters and animate initial keyboard presses until release.
func newGrabCmd() *cobra.Command {
	return newGrabCmdWithDependencies(defaultGrabDependencies())
}

func newGrabCmdWithDependencies(dependencies grabDependencies) *cobra.Command {
	var matchExprs []string
	var excludeExprs []string

	cmd := &cobra.Command{
		Use:   "grab",
		Short: "Grab input devices and turn key presses into animations.",
		Long: `Grab Linux input devices and show each initial keyboard press as a
colorful firework animation until the process receives SIGINT / SIGTERM.

Filters are written as repeatable --match and --exclude expressions of
the form key=value:

  path=/dev/input/by-id/usb-Logitech_*-event-kbd   path glob
  name=Logitech                                    case-insensitive substring
  type=keyboard                                    keyboard | mouse | touchpad | gamepad | other

A device is grabbed when every --match succeeds and no --exclude
succeeds. With no filters, every readable input device is grabbed.

Animation requires an interactive, color-capable terminal. Printable keys use
a US keyboard layout; special, media, and modifier keys use named labels.
Holding a key creates one effect: releases and autorepeat do not create effects.
Press Ctrl+Alt+Esc to release the session.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			matchers, err := capture.ParseMatchers(matchExprs)
			if err != nil {
				return err
			}
			excludes, err := capture.ParseMatchers(excludeExprs)
			if err != nil {
				return err
			}

			// Renderer construction performs terminal preflight without emitting
			// escape sequences. It must happen before NewSession grabs devices.
			renderer, err := dependencies.newRenderer(cmd.OutOrStdout())
			if err != nil {
				return err
			}

			session, err := dependencies.newSession(ctx, capture.Options{
				Matchers: matchers,
				Excludes: excludes,
				Logger:   LoggerFromContext(ctx),
				OnEvent:  renderer.Submit,
			})
			if err != nil {
				return errors.Join(err, renderer.Close())
			}

			return runGrab(ctx, renderer, session)
		},
	}
	cmd.Flags().StringSliceVar(&matchExprs, "match", nil,
		"Match expression (key=value, repeatable). Keys: path, name, type.")
	cmd.Flags().StringSliceVar(&excludeExprs, "exclude", nil,
		"Exclude expression (key=value, repeatable). Same syntax as --match.")
	return cmd
}

type grabResult struct {
	component string
	err       error
}

func runGrab(ctx context.Context, renderer grabRenderer, session capture.Session) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan grabResult, 2)
	go func() { results <- grabResult{component: "animation", err: renderer.Run(runCtx)} }()
	go func() { results <- grabResult{component: "capture", err: session.Run(runCtx)} }()

	first := <-results
	cancel()
	second := <-results

	closeErr := session.Close(context.WithoutCancel(ctx))
	restoreErr := renderer.Close()

	var runErrors []error
	for _, result := range []grabResult{first, second} {
		if result.err != nil && !errors.Is(result.err, context.Canceled) {
			runErrors = append(runErrors, fmt.Errorf("%s: %w", result.component, result.err))
		}
	}
	if closeErr != nil {
		runErrors = append(runErrors, fmt.Errorf("release devices: %w", closeErr))
	}
	if restoreErr != nil {
		runErrors = append(runErrors, fmt.Errorf("restore terminal: %w", restoreErr))
	}
	return errors.Join(runErrors...)
}
