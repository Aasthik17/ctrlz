package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff [path] <a> [b]",
	Short: "Show a diff between snapshots",
	Long:  "Diffs snapshot a against snapshot b, or against the current working tree if b is omitted. Prints a standard unified diff to stdout.",
	Args:  cobra.RangeArgs(1, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("not implemented yet")
		return nil
	},
}
