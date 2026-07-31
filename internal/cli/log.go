package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var logLimit int

var logCmd = &cobra.Command{
	Use:   "log [path]",
	Short: "Show the snapshot history",
	Long:  "Prints snapshots for path (default: cwd) newest-first: short hash, relative time, reason tag, and a one-line file-change stat.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("not implemented yet")
		return nil
	},
}

func init() {
	logCmd.Flags().IntVar(&logLimit, "limit", 20, "maximum number of snapshots to show")
}
