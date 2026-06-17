// Package feedback implements `gh wheel feedback`, a TUI form for filing
// feature requests and bug reports against gh-wheel's own repository.
package feedback

import (
	"fmt"
	"os"

	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"github.com/UtakataKyosui/gh-wheel/internal/cliexit"
	"github.com/UtakataKyosui/gh-wheel/internal/ghclient"
	"github.com/UtakataKyosui/gh-wheel/internal/issuereport"
)

// NewCmd returns the `gh wheel feedback` subcommand.
func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "feedback",
		Short: "Submit a feature request or bug report for gh-wheel",
		Long: `Open an interactive TUI form to file a feature request or bug report
against the gh-wheel repository on GitHub.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
				return cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs,
					fmt.Errorf("feedback requires an interactive terminal"))
			}

			result, err := runFeedbackTUI()
			if err != nil {
				return cliexit.NewGeneral(fmt.Errorf("TUI error: %w", err))
			}
			if result == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "キャンセルしました。")
				return nil
			}

			client, err := ghclient.NewForRepo(issuereport.ReportOwner, issuereport.ReportRepo)
			if err != nil {
				return err
			}

			url, err := submitFeedback(client, result)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Issue を作成しました: %s\n", url)
			return nil
		},
	}
}

// submitFeedback posts the feedback as a GitHub Issue and returns the new issue URL.
func submitFeedback(c *ghclient.Client, f *feedbackResult) (string, error) {
	label := "enhancement"
	if f.kind == kindBug {
		label = "bug"
	}
	payload := map[string]any{
		"title":  f.title,
		"body":   f.body,
		"labels": []string{label},
	}
	var resp struct {
		HTMLURL string `json:"html_url"`
	}
	if err := c.RepoPost("issues", payload, &resp); err != nil {
		return "", err
	}
	return resp.HTMLURL, nil
}
