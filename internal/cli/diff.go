package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Aasthik17/ctrlz/internal/store"
)

var diffCmd = &cobra.Command{
	Use:   "diff [path] <a> [b]",
	Short: "Show a diff between snapshots",
	Long:  "Diffs snapshot a against snapshot b, or against the current working tree if b is omitted. Prints a standard unified diff to stdout.",
	Args:  cobra.RangeArgs(1, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, a, b := parseDiffArgs(args)

		project, err := store.Lookup(path)
		if err != nil {
			return err
		}

		storePath, err := store.ProjectStorePath(project.ID)
		if err != nil {
			return err
		}

		out, err := store.Diff(storePath, project.Path, a, b)
		if err != nil {
			return err
		}
		fmt.Print(out)
		return nil
	},
}

// parseDiffArgs splits the positional args into (path, a, b). path is
// optional and always comes first, but since it's followed by 1-2 required
// refs, a 2-arg invocation is ambiguous between (path, a) and (a, b). It's
// resolved by checking whether the first argument is an existing directory:
// git refs never are.
func parseDiffArgs(args []string) (path, a, b string) {
	path = "."
	switch len(args) {
	case 1:
		a = args[0]
	case 2:
		if isDir(args[0]) {
			path, a = args[0], args[1]
		} else {
			a, b = args[0], args[1]
		}
	case 3:
		path, a, b = args[0], args[1], args[2]
	}
	return path, a, b
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
