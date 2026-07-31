package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/Aasthik17/ctrlz/internal/store"
	"github.com/Aasthik17/ctrlz/internal/watch"
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
		path, command, err := parseWatchArgs(cmd, args)
		if err != nil {
			return err
		}

		interval, err := time.ParseDuration(watchInterval)
		if err != nil {
			return fmt.Errorf("invalid --interval %q: %w", watchInterval, err)
		}

		project, _, err := store.LookupOrCreate(path)
		if err != nil {
			return err
		}

		storePath, err := store.ProjectStorePath(project.ID)
		if err != nil {
			return err
		}

		lockPath, err := store.WatchLockPath(project.ID)
		if err != nil {
			return err
		}

		release, err := watch.AcquireLock(lockPath)
		if err != nil {
			return err
		}
		defer release()

		exitCode, err := watch.Run(watch.Options{
			StorePath: storePath,
			WorkTree:  project.Path,
			Interval:  interval,
			Quiet:     watchQuiet,
			Command:   command,
		})
		if err != nil {
			return err
		}
		if exitCode != 0 {
			os.Exit(exitCode)
		}
		return nil
	},
}

func init() {
	watchCmd.Flags().StringVar(&watchInterval, "interval", "5s", "how often to check for changes and snapshot")
	watchCmd.Flags().BoolVar(&watchQuiet, "quiet", false, "suppress the startup line as well as per-tick output")
}

// parseWatchArgs splits args around a "--" separator (which cobra strips,
// reporting its position via ArgsLenAtDash) into an optional leading path
// and a trailing command to wrap.
func parseWatchArgs(cmd *cobra.Command, args []string) (path string, command []string, err error) {
	path = "."

	dash := cmd.ArgsLenAtDash()
	if dash < 0 {
		if len(args) > 1 {
			return "", nil, fmt.Errorf("too many arguments (did you mean to put -- before a command?)")
		}
		if len(args) == 1 {
			path = args[0]
		}
		return path, nil, nil
	}

	pathArgs, command := args[:dash], args[dash:]
	if len(pathArgs) > 1 {
		return "", nil, fmt.Errorf("too many arguments before --: %v", pathArgs)
	}
	if len(pathArgs) == 1 {
		path = pathArgs[0]
	}
	if len(command) == 0 {
		return "", nil, fmt.Errorf("-- must be followed by a command to run")
	}
	return path, command, nil
}
