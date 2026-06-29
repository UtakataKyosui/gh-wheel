// Package skill implements `gh wheel skill`, which generates a Claude Code
// Agent Skill (SKILL.md) describing how to operate gh-wheel.
package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/spf13/cobra"

	"github.com/UtakataKyosui/gh-wheel/internal/cliexit"
	"github.com/UtakataKyosui/gh-wheel/internal/skillgen"
)

// nameRe constrains --name to a safe kebab-case identifier. It rejects path
// separators and "." segments, preventing path traversal in --output mode.
var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type opts struct {
	name        string
	description string
	output      string
}

// NewCmd returns the `gh wheel skill` subcommand.
func NewCmd() *cobra.Command {
	var o opts

	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Generate a Claude Code Agent Skill (SKILL.md) for operating gh-wheel",
		Long: `Generate a Claude Code Agent Skill that documents every gh-wheel command,
flag, and behavior so an AI agent can operate gh-wheel.

The skill is built by introspecting the live command tree, so it stays in sync
with the actual commands.

By default the SKILL.md is written to stdout. Pass --output <dir> to write it to
<dir>/<name>/SKILL.md (e.g. --output .claude/skills produces
.claude/skills/gh-wheel/SKILL.md).`,
		Deprecated: "the Claude Code skill for gh-wheel is now published via gh skill.\n\nInstall it with:\n  gh skill install UtakataKyosui/gh-wheel gh-wheel\n",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !nameRe.MatchString(o.name) {
				return cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs,
					fmt.Errorf("invalid --name %q: must match %s (kebab-case, no path separators)",
						o.name, nameRe.String()))
			}

			content := skillgen.Generate(cmd.Root(), skillgen.Options{
				Name:        o.name,
				Description: o.description,
			})

			if o.output == "" {
				fmt.Fprint(cmd.OutOrStdout(), content)
				return nil
			}

			dir := filepath.Join(o.output, o.name)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return cliexit.NewGeneral(fmt.Errorf("create skill directory %s: %w", dir, err))
			}
			path := filepath.Join(dir, "SKILL.md")
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return cliexit.NewGeneral(fmt.Errorf("write %s: %w", path, err))
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "wrote %s\n", path)
			return nil
		},
	}

	cmd.Flags().StringVar(&o.name, "name", "gh-wheel", "Skill name (frontmatter name and output directory)")
	cmd.Flags().StringVar(&o.description, "description", "", "Override the frontmatter description")
	cmd.Flags().StringVarP(&o.output, "output", "o", "", "Directory to write <name>/SKILL.md into (default: stdout)")

	return cmd
}
