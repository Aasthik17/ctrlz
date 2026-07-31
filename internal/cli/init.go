package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Aasthik17/ctrlz/internal/store"
)

var initCmd = &cobra.Command{
	Use:   "init [path]",
	Short: "Register a directory with ctrlz",
	Long:  "Registers path (default: cwd) in the ctrlz registry and creates its bare snapshot store if one doesn't exist. Idempotent.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) == 1 {
			path = args[0]
		}

		project, created, err := store.LookupOrCreate(path)
		if err != nil {
			return err
		}

		if created {
			fmt.Printf("ctrlz now tracking %s (project %s)\n", project.Path, project.ID)
		} else {
			fmt.Printf("ctrlz already tracking %s (project %s)\n", project.Path, project.ID)
		}
		return nil
	},
}
