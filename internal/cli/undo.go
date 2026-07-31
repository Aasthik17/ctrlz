package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	undoTo  string
	undoYes bool
)

var undoCmd = &cobra.Command{
	Use:   "undo [path]",
	Short: "Revert a directory to a prior snapshot",
	Long:  "Takes a pre-undo safety snapshot of the current state, then reverts path (default: cwd) to the most recent snapshot, or to --to <ref> if given.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("not implemented yet")
		return nil
	},
}

func init() {
	undoCmd.Flags().StringVar(&undoTo, "to", "", "revert to a specific snapshot ref instead of the most recent one")
	undoCmd.Flags().BoolVar(&undoYes, "yes", false, "skip the confirmation prompt")
}
