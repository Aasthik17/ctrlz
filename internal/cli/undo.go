package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Aasthik17/ctrlz/internal/store"
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

		plan, err := store.PrepareUndo(storePath, project.Path, undoTo)
		if err != nil {
			return err
		}

		if plan.PreUndoTaken {
			fmt.Printf("Saved current state first: %s (pre-undo)\n", shortHash(plan.PreUndoHash))
		}
		fmt.Printf(
			"Reverting to %s will restore %d file(s), remove %d file(s), and modify %d file(s).\n",
			shortHash(plan.Target), plan.Summary.ToRestore, plan.Summary.ToRemove, plan.Summary.ToModify,
		)

		if plan.Summary.ToRestore == 0 && plan.Summary.ToRemove == 0 && plan.Summary.ToModify == 0 {
			fmt.Println("Already up to date, nothing to do.")
			return nil
		}

		if !undoYes && !confirm("Proceed?") {
			fmt.Println("Aborted.")
			return nil
		}

		if err := store.ApplyUndo(storePath, project.Path, plan.Target); err != nil {
			return err
		}

		fmt.Printf("Reverted %s to %s.\n", project.Path, shortHash(plan.Target))
		return nil
	},
}

func init() {
	undoCmd.Flags().StringVar(&undoTo, "to", "", "revert to a specific snapshot ref instead of the most recent one")
	undoCmd.Flags().BoolVar(&undoYes, "yes", false, "skip the confirmation prompt")
}

func shortHash(hash string) string {
	if len(hash) > 12 {
		return hash[:12]
	}
	return hash
}

func confirm(prompt string) bool {
	fmt.Printf("%s [y/N] ", prompt)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}
