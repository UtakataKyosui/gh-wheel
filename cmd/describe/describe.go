// Package describe implements the `gh wheel describe` subcommand,
// which prints gh-wheel's command schema as machine-readable JSON.
package describe

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

type exitCodeEntry struct {
	Code        int    `json:"code"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

type commandEntry struct {
	Name       string `json:"name"`
	Short      string `json:"short"`
	OutputKind string `json:"output_kind,omitempty"`
}

type commandSchema struct {
	SchemaVersion string          `json:"schema_version"`
	Kind          string          `json:"kind"`
	Name          string          `json:"name"`
	Short         string          `json:"short"`
	Commands      []commandEntry  `json:"commands"`
	ExitCodes     []exitCodeEntry `json:"exit_codes"`
}

var spec = commandSchema{
	SchemaVersion: "v1",
	Kind:          "command_schema",
	Name:          "wheel",
	Short:         "A unified gh extension for Issue-Driven development",
	Commands: []commandEntry{
		{Name: "task", Short: "Manage your GitHub tasks (PRs and Issues)", OutputKind: "task_result"},
		{Name: "task time", Short: "Show estimated work time breakdown for a PR by session", OutputKind: "task_time_result"},
		{Name: "task next", Short: "Find an approachable unstarted Issue and self-assign", OutputKind: "task_next_result"},
		{Name: "graph", Short: "Visualize GitHub Issue/PR relationship graphs", OutputKind: "graph_result"},
		{Name: "monitor", Short: "Watch multiple repos in a live TUI"},
		{Name: "review", Short: "AI-assisted code review workflows"},
		{Name: "okr", Short: "Compute GitHub activity metrics for OKR key results"},
		{Name: "okr metrics", Short: "Compute period GitHub metrics (cross-repo) for okr-hub KR sync", OutputKind: "okr_metrics"},
		{Name: "feedback", Short: "Send feature requests or bug reports for gh-wheel"},
		{Name: "skill", Short: "Generate a Claude Code Agent Skill scaffold"},
		{Name: "describe", Short: "Print gh-wheel's command schema as JSON"},
	},
	ExitCodes: []exitCodeEntry{
		{Code: 0, Category: "success", Description: "Command completed successfully"},
		{Code: 1, Category: "general", Description: "Internal or unexpected error"},
		{Code: 2, Category: "usage", Description: "Invalid arguments or flags"},
		{Code: 3, Category: "not_found", Description: "Requested resource not found"},
		{Code: 4, Category: "auth", Description: "Authentication required or token expired"},
		{Code: 5, Category: "validation", Description: "Input validation failed"},
		{Code: 6, Category: "api", Description: "GitHub API request failed"},
	},
}

// Write serialises the command schema to w as indented JSON.
func Write(w io.Writer) error {
	b, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return fmt.Errorf("describe marshal: %w", err)
	}
	_, err = fmt.Fprintf(w, "%s\n", b)
	return err
}

// NewCmd returns the `gh wheel describe` subcommand.
func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "describe",
		Short: "Print gh-wheel's command schema as JSON",
		Long: `Print gh-wheel's full command schema as machine-readable JSON.

The output includes the list of subcommands, their output kinds, and
the complete exit code table with categories — making it easy for AI
agents and scripts to understand the CLI contract without parsing --help.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return Write(os.Stdout)
		},
	}
}
