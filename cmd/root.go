package cmd

import (
	"github.com/UtakataKyosui/gh-wheel/cmd/graph"
	"github.com/UtakataKyosui/gh-wheel/cmd/monitor"
	"github.com/UtakataKyosui/gh-wheel/cmd/review"
	"github.com/UtakataKyosui/gh-wheel/cmd/task"
	"github.com/spf13/cobra"
)

// NewRootCmd builds the root cobra command for `gh wheel`.
//
// Auth gate (PersistentPreRunE calling internal/auth) and full persistent-flag
// semantics are wired in Issue #3 once internal/auth and internal/ghclient are
// available.
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
	}

	// Persistent flags — auth gate and semantics wired in Issue #3
	// (internal/auth, internal/ghclient, internal/cliexit, internal/jsonout).
	root.PersistentFlags().StringP("repo", "R", "", "Repository in owner/repo format (detected from cwd if omitted)")
	root.PersistentFlags().BoolP("json", "j", false, "Output results as JSON")
	root.PersistentFlags().Bool("dry-run", false, "Validate input without sending API requests")
	root.PersistentFlags().String("jq", "", "Filter JSON output with a jq expression")

	root.AddCommand(
		task.NewCmd(),
		graph.NewCmd(),
		monitor.NewCmd(),
		review.NewCmd(),
	)

	return root
}
