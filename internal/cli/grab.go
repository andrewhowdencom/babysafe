package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	evdev "github.com/holoplot/go-evdev"

	"github.com/andrewhowdencom/babysafe/pkg/capture"
)

// newGrabCmd is the workhorse command: grab every input device that
// matches the supplied --match / --exclude expressions and hold them
// until the process is signalled. Match expressions are repeatable;
// all --match rules must be satisfied and no --exclude rule may be.
//
// Examples (see --help for the full list):
//
//	sudo babysafe grab --match type=keyboard
//	sudo babysafe grab --match name=Logitech
//	sudo babysafe grab --match path=/dev/input/by-id/usb-Logitech_*-event-kbd
func newGrabCmd() *cobra.Command {
	var matchExprs []string
	var excludeExprs []string
	var echo bool

	cmd := &cobra.Command{
		Use:   "grab",
		Short: "Grab input devices and hold them until interrupted.",
		Long: `Grab Linux input devices and hold them until the process receives
SIGINT / SIGTERM.

Filters are written as repeatable --match and --exclude expressions of
the form key=value:

  path=/dev/input/by-id/usb-Logitech_*-event-kbd   path glob
  name=Logitech                                    case-insensitive substring
  type=keyboard                                    keyboard | mouse | touchpad | gamepad | other

A device is grabbed when every --match succeeds and no --exclude
succeeds. With no filters, every readable input device is grabbed.

By default, the keys the grab catches are decoded and printed to
stdout as they would appear at the keyboard — useful for seeing
what a child is typing without releasing the grab. Special keys
(Enter, Tab, arrow keys, F-keys, …) are shown as bracketed names
like [ENTER]. The decoder assumes a US keyboard layout; non-US
layouts will mis-decode. Pass --echo=false to suppress this output.
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

			var onEvent func(*evdev.InputEvent)
			if echo {
				d := &echoDecoder{}
				onEvent = func(ev *evdev.InputEvent) {
					if s := d.feed(ev); s != "" {
						// Write to the command's stdout so the
						// output is redirectable / testable like
						// any other CLI tool's output.
						fmt.Fprint(cmd.OutOrStdout(), s)
					}
				}
			}

			logger := LoggerFromContext(ctx)
			sess, err := capture.NewSession(ctx, capture.Options{
				Matchers: matchers,
				Excludes: excludes,
				Logger:   logger,
				OnEvent:  onEvent,
			})
			if err != nil {
				return err
			}
			defer func() {
				if cerr := sess.Close(context.Background()); cerr != nil {
					cmd.PrintErrf("release: %v\n", cerr)
				}
			}()

			paths := sess.Devices()
			fmt.Fprint(cmd.OutOrStdout(), holdingMessage(len(paths)))
			for _, p := range paths {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", p)
			}

			return sess.Run(ctx)
		},
	}
	cmd.Flags().StringSliceVar(&matchExprs, "match", nil,
		"Match expression (key=value, repeatable). Keys: path, name, type.")
	cmd.Flags().StringSliceVar(&excludeExprs, "exclude", nil,
		"Exclude expression (key=value, repeatable). Same syntax as --match.")
	cmd.Flags().BoolVar(&echo, "echo", true,
		"Print each key event to stdout as the user would type it. Default true; pass --echo=false to disable.")

	return cmd
}

func holdingMessage(deviceCount int) string {
	return fmt.Sprintf(
		"babysafe: holding %d device(s); press Ctrl+Alt+Esc to release.\n",
		deviceCount,
	)
}

// joinTypes re-used by list.go — keep a local alias so neither file
// owns the helper.
var _ = strings.Join
