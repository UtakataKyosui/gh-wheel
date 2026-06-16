package task

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/UtakataKyosui/gh-wheel/internal/cliexit"
	"github.com/UtakataKyosui/gh-wheel/internal/ghclient"
	"github.com/UtakataKyosui/gh-wheel/internal/jsonout"
)

// NewCmd returns the `gh wheel task` subcommand.
// Child commands (prompt, close, schedule) are added in Issues #5–#8.
func NewCmd() *cobra.Command {
	var opts fetchOpts

	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage your GitHub tasks (PRs and Issues)",
		Long:  "Browse and operate on PRs and Issues you are involved in as author or reviewer.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.State != "open" && opts.State != "closed" && opts.State != "all" {
				return cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs,
					fmt.Errorf("invalid state %q: must be one of open, closed, all", opts.State))
			}

			if opts.AuthorOnly && opts.ReviewOnly {
				return cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs,
					fmt.Errorf("--author-only and --review-only are mutually exclusive"))
			}

			flagRepo, _ := cmd.Flags().GetString("repo")
			jsonOut, _ := cmd.Flags().GetBool("json")
			jqExpr, _ := cmd.Flags().GetString("jq")

			c, err := ghclient.New(flagRepo)
			if err != nil {
				return err
			}

			login, err := c.CurrentUser()
			if err != nil {
				return err
			}

			result, err := fetch(c, login, opts)
			if err != nil {
				return err
			}

			if jsonOut {
				return jsonout.Print(result, jqExpr)
			}

			printTable(os.Stdout, result)
			return nil
		},
	}

	cmd.Flags().StringVarP(&opts.State, "state", "s", "open", "Filter by state: open, closed, all")
	cmd.Flags().BoolVarP(&opts.AuthorOnly, "author-only", "a", false, "Show only PRs where you are the author")
	cmd.Flags().BoolVarP(&opts.ReviewOnly, "review-only", "r", false, "Show only PRs where review is requested from you")
	cmd.Flags().BoolVarP(&opts.IncludeDrafts, "include-drafts", "d", true, "Include draft PRs")
	cmd.Flags().BoolVarP(&opts.WithIssues, "with-issues", "I", false, "Include Issues assigned to you")
	cmd.Flags().BoolVar(&opts.IssuesOnly, "issues-only", false, "Show only Issues (implies --with-issues)")
	cmd.Flags().BoolVar(&opts.WithReviews, "with-reviews", false, "Fetch review status for each PR (slower)")

	cmd.AddCommand(newPromptCmd())

	return cmd
}
