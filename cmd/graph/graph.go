package graph

import "github.com/spf13/cobra"

// NewCmd returns the `gh wheel graph` subcommand.
// Child commands (list, tree, dot) are added in Issues #9–#10.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graph",
		Short: "Visualize GitHub Issue/PR relationship graphs",
		Long:  "Fetch and display dependency and reference graphs between Issues and PRs.",
	}
	return cmd
}
