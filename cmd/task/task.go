package task

import "github.com/spf13/cobra"

// NewCmd returns the `gh wheel task` subcommand.
// Child commands (list, prompt, close, schedule) are added in Issues #4–#8.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage your GitHub tasks (PRs and Issues)",
		Long:  "Browse and operate on PRs and Issues you are involved in as author or reviewer.",
	}
	return cmd
}
