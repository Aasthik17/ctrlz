package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Aasthik17/ctrlz/internal/store"
)

var logLimit int

var logCmd = &cobra.Command{
	Use:   "log [path]",
	Short: "Show the snapshot history",
	Long:  "Prints snapshots for path (default: cwd) newest-first: short hash, relative time, reason tag, and a one-line file-change stat.",
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

		snapshots, err := store.ListSnapshots(storePath, logLimit)
		if err != nil {
			return err
		}
		if len(snapshots) == 0 {
			fmt.Println("no snapshots yet")
			return nil
		}

		for _, s := range snapshots {
			fmt.Printf("%s  %-14s  %-8s  %s\n", s.Hash[:12], relativeTime(s.Timestamp), s.Reason, s.Stat)
		}
		return nil
	},
}

func init() {
	logCmd.Flags().IntVar(&logLimit, "limit", 20, "maximum number of snapshots to show")
}
