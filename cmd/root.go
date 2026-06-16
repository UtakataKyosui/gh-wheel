package cmd

import (
	"github.com/UtakataKyosui/gh-wheel/cmd/graph"
	"github.com/UtakataKyosui/gh-wheel/cmd/monitor"
	"github.com/UtakataKyosui/gh-wheel/cmd/review"
	"github.com/UtakataKyosui/gh-wheel/cmd/skill"
	"github.com/UtakataKyosui/gh-wheel/cmd/task"
	"github.com/UtakataKyosui/gh-wheel/internal/auth"
	"github.com/spf13/cobra"
)

// NewRootCmd builds the root cobra command for `gh wheel`.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "wheel",
		Short: "A unified gh extension for Issue-Driven development",
		Long: `gh-wheel integrates task management, issue relationship graphs,
and code review workflows into a single gh extension.

  gh wheel task     — browse and manage your PRs and Issues
  gh wheel graph    — visualize Issue/PR dependency graphs
  gh wheel monitor  — watch multiple repos in a live TUI
  gh wheel review   — AI-assisted code review workflows`,
		SilenceErrors: true,
		SilenceUsage:  true,
		// PersistentPreRunE gates every subcommand on gh presence and version.
		// Help, completion, and --help bypass the gate to avoid spurious errors.
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			switch cmd.Name() {
			case "help", "__complete", "__completeNoDesc":
				return nil
			}
			if cmd.Flags().Changed("help") {
				return nil
			}
			if err := auth.CheckGH(); err != nil {
				return err
			}
			return auth.CheckGHVersion()
		},
	}

	// Persistent flags — read by subcommands via root.PersistentFlags().
	root.PersistentFlags().StringP("repo", "R", "", "Repository in owner/repo format (detected from cwd if omitted)")
	root.PersistentFlags().BoolP("json", "j", false, "Output results as JSON")
	root.PersistentFlags().Bool("dry-run", false, "Validate input without sending API requests")
	root.PersistentFlags().String("jq", "", "Filter JSON output with a jq expression")

	root.AddCommand(
		task.NewCmd(),
		graph.NewCmd(),
		monitor.NewCmd(),
		review.NewCmd(),
		skill.NewCmd(),
	)

	return root
}
