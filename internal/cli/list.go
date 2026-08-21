package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/andrewhowdencom/babysafe/pkg/capture"
)

// newListCmd prints every input device the kernel exposes, with the
// device's friendly name and the categories babysafe inferred from
// its capabilities. Listing is read-only and does not require root —
// unreadable devices are silently skipped (logged at warn level).
func newListCmd() *cobra.Command {
	var showPathsOnly bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List input devices visible to the kernel.",
		Long: `List input devices visible to the kernel.

Each row reports the device path (/dev/input/eventN), the friendly
name returned by EVIOCGNAME, and the categories babysafe inferred from
the device's event capabilities (keyboard, mouse, touchpad, gamepad,
other). Devices that the process cannot open (typically because the
process is not running as root) are skipped; re-run with sudo to see
everything.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			devs, err := capture.ListDevices(ctx, LoggerFromContext(ctx))
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()
			if showPathsOnly {
				for _, d := range devs {
					fmt.Fprintln(w, d.Path)
				}
				return nil
			}

			// Compute column widths from the data so long names don't
			// push subsequent columns off-screen.
			pathW, nameW := len("PATH"), len("NAME")
			for _, d := range devs {
				if l := len(d.Path); l > pathW {
					pathW = l
				}
				if l := len(d.Name); l > nameW {
					nameW = l
				}
			}

			fmt.Fprintf(w, "%-*s  %-*s  TYPES\n", pathW, "PATH", nameW, "NAME")
			for _, d := range devs {
				fmt.Fprintf(w, "%-*s  %-*s  %s\n",
					pathW, d.Path, nameW, d.Name, joinTypes(d.Types))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&showPathsOnly, "paths-only", false, "Print one path per line; skip the header and types column.")
	return cmd
}

// joinTypes renders a Device's types as a comma-separated string. An
// empty slice renders as "-" so the column doesn't vanish.
func joinTypes(types []capture.DeviceType) string {
	if len(types) == 0 {
		return "-"
	}
	parts := make([]string, len(types))
	for i, t := range types {
		parts[i] = string(t)
	}
	return strings.Join(parts, ",")
}
