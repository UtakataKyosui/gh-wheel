package review

import "github.com/spf13/cobra"

// NewCmd returns the `gh wheel review` subcommand.
// Child commands (schema, prompt, validate, post, reply, threads) are added in Issues #11–#14.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Code review workflows for GitHub PRs",
		Long:  "Generate review prompts, validate AI review output, and post structured code reviews.",
	}
	return cmd
}
