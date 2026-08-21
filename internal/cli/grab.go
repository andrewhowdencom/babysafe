package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

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
succeeds. With no filters, every readable input device is grabbed.`,
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

			logger := LoggerFromContext(ctx)
			sess, err := capture.NewSession(ctx, capture.Options{
				Matchers: matchers,
				Excludes: excludes,
				Logger:   logger,
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
			fmt.Fprintf(cmd.OutOrStdout(),
				"babysafe: holding %d device(s); press Ctrl-C to release.\n",
				len(paths))
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

	return cmd
}

// joinTypes re-used by list.go — keep a local alias so neither file
// owns the helper.
var _ = strings.Join
