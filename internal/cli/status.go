package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Aasthik17/ctrlz/internal/store"
	"github.com/Aasthik17/ctrlz/internal/watch"
)

var statusCmd = &cobra.Command{
	Use:   "status [path]",
	Short: "Show project id, snapshot count, and watch state",
	Long:  "Prints project id, total snapshot count, store size on disk, time of the most recent snapshot, and whether a watch is currently active for path (default: cwd).",
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

		count, err := store.SnapshotCount(storePath)
		if err != nil {
			return err
		}

		size, err := store.StoreSize(storePath)
		if err != nil {
			return err
		}

		latest, err := store.ListSnapshots(storePath, 1)
		if err != nil {
			return err
		}

		lockPath, err := store.WatchLockPath(project.ID)
		if err != nil {
			return err
		}
		active, _, err := watch.Status(lockPath)
		if err != nil {
			return err
		}

		fmt.Printf("project:       %s\n", project.ID)
		fmt.Printf("path:          %s\n", project.Path)
		fmt.Printf("snapshots:     %d\n", count)
		fmt.Printf("store size:    %s\n", humanSize(size))
		if len(latest) == 0 {
			fmt.Println("last snapshot: never")
		} else {
			fmt.Printf("last snapshot: %s\n", relativeTime(latest[0].Timestamp))
		}
		if active {
			fmt.Println("watch:         active")
		} else {
			fmt.Println("watch:         not active")
		}
		return nil
	},
}
