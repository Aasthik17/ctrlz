package cli

import (
	"github.com/spf13/cobra"

	"github.com/Aasthik17/ctrlz/internal/store"
	"github.com/Aasthik17/ctrlz/internal/tui"
)

var tuiCmd = &cobra.Command{
	Use:   "tui [path]",
	Short: "Browse snapshots and safely restore them interactively",
	Long:  "Opens a full-screen interactive timeline for path (default: cwd). Select a snapshot to review its diff against the current working tree, then restore it with typed-hash confirmation.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) == 1 {
			path = args[0]
		}
		project, err := store.Lookup(path)
		if err != nil {
			return err
		}
		storePath, err := store.ProjectStorePath(project.ID)
		if err != nil {
			return err
		}
		return tui.Run(project, storePath)
	},
}
