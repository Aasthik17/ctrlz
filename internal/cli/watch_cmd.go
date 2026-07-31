package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	watchInterval string
	watchQuiet    bool
)

var watchCmd = &cobra.Command{
	Use:   "watch [path] [-- command...]",
	Short: "Watch a directory and take automatic snapshots",
	Long:  "Watches path (default: cwd) and takes an automatic snapshot every --interval when something changes. If a command follows --, runs it as a foreground subprocess and stops watching when it exits.",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("not implemented yet")
		return nil
	},
}

func init() {
	watchCmd.Flags().StringVar(&watchInterval, "interval", "5s", "how often to check for changes and snapshot")
	watchCmd.Flags().BoolVar(&watchQuiet, "quiet", false, "suppress the startup line as well as per-tick output")
}
