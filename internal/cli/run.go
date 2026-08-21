package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/andrewhowdencom/babysafe/internal/app"
	"github.com/andrewhowdencom/babysafe/internal/config"
)

// newRunCmd is the workhorse command: grab every Linux input device
// and hold them until the user (or a signal) tells us to stop. At the
// skeleton stage it just confirms configuration loaded and that the
// application can grab devices.
func newRunCmd(cfg *config.Config) *cobra.Command {
	var releaseOnExit bool

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Capture input devices and hold them until interrupted.",
		Long: `Run babysafe: grab every available input device on the host and
hold it until the process is signalled. Devices are released
automatically on SIGINT / SIGTERM, or held if --release-on-exit=false
is set.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			a, err := app.New(ctx, cfg, app.Options{ReleaseOnExit: releaseOnExit})
			if err != nil {
				return fmt.Errorf("construct application: %w", err)
			}
			defer func() {
				if err := a.Shutdown(context.Background()); err != nil {
					cmd.PrintErrf("shutdown failed: %v\n", err)
				}
			}()

			cmd.Printf("babysafe: capturing %d input device(s); press Ctrl-C to release.\n", a.DeviceCount())
			return a.Run(ctx)
		},
	}

	cmd.Flags().BoolVar(&releaseOnExit, "release-on-exit", true, "Release captured devices when the process exits cleanly.")

	return cmd
}
