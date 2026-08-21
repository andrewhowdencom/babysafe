package cli

import (
	"github.com/spf13/cobra"

	"github.com/andrewhowdencom/babysafe/internal/version"
)

// newVersionCmd reports the binary's version. The version string is
// stamped in via Taskfile.yml at build time using -ldflags.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the babysafe version and exit.",
		Long:  "Print the babysafe version and exit. The version is set at build time via -ldflags.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.Println(version.String())
			return nil
		},
	}
}
