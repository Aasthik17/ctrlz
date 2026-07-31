package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [path]",
	Short: "Show project id, snapshot count, and watch state",
	Long:  "Prints project id, total snapshot count, store size on disk, time of the most recent snapshot, and whether a watch is currently active for path (default: cwd).",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("not implemented yet")
		return nil
	},
}
