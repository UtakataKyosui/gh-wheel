package monitor

import "github.com/spf13/cobra"

// NewCmd returns the `gh wheel monitor` subcommand.
// TUI implementation is added in Issue #16.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "Monitor multiple repositories in a TUI dashboard",
		Long:  "Watch multiple GitHub repositories for new Issues and PRs in real time.",
	}
	return cmd
}
