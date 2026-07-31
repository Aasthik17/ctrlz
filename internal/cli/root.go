// Package cli wires up the ctrlz command-line interface.
package cli

import (
	"github.com/spf13/cobra"
)

// rootCmd is the base command; every subcommand attaches to this.
var rootCmd = &cobra.Command{
	Use:   "ctrlz",
	Short: "An undo button for AI coding agents",
	Long: `ctrlz is an undo button for AI coding agents.

It runs underneath any coding agent (Claude Code, Cursor, Codex CLI, Cline,
Replit's agent, whatever), takes automatic snapshots of a working directory
as the agent operates, and gives an instant, zero-config way to revert to
any prior point. It does not sandbox or restrict the agent. It guarantees
recovery instead of trying to predict danger in advance.`,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(undoCmd)
	rootCmd.AddCommand(logCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(statusCmd)
}
