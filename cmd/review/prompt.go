package review

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/UtakataKyosui/gh-wheel/internal/cliexit"
	"github.com/UtakataKyosui/gh-wheel/internal/ghclient"
	"github.com/UtakataKyosui/gh-wheel/internal/reviewschema"
)

// prMeta maps fields returned by GET /repos/{owner}/{repo}/pulls/{number}.
type prMeta struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	User   struct {
		Login string `json:"login"`
	} `json:"user"`
	ChangedFiles int `json:"changed_files"`
	Head         struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

// prFile maps a single entry from GET /repos/{owner}/{repo}/pulls/{number}/files.
type prFile struct {
	Filename string `json:"filename"`
	Patch    string `json:"patch"`
}

const diffMaxBytes = 80_000

// newPromptCmd returns the `gh wheel review prompt <PR>` subcommand.
func newPromptCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prompt <PR>",
		Short: "Output a Markdown review prompt for a PR to stdout",
		Long: `Fetch PR metadata and diff, then write a Markdown prompt suitable
for AI review to stdout.

Example:
  gh wheel review prompt 123 | claude --print > review.json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prNum, err := strconv.Atoi(args[0])
			if err != nil || prNum <= 0 {
				return cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs,
					fmt.Errorf("invalid PR number %q: must be a positive integer", args[0]))
			}

			flagRepo, _ := cmd.Flags().GetString("repo")
			c, err := ghclient.New(flagRepo)
			if err != nil {
				return err
			}

			var meta prMeta
			if err := c.RepoGet(fmt.Sprintf("pulls/%d", prNum), &meta); err != nil {
				return err
			}

			var files []prFile
			if err := c.RepoGet(fmt.Sprintf("pulls/%d/files?per_page=100", prNum), &files); err != nil {
				return err
			}

			diff := buildDiff(files)
			renderPrompt(os.Stdout, meta, diff)
			return nil
		},
	}
	return cmd
}

// buildDiff concatenates per-file patches into a unified diff string, truncating
// at diffMaxBytes and appending a truncation marker if the content exceeds the limit.
func buildDiff(files []prFile) string {
	var sb strings.Builder
	for _, f := range files {
		header := fmt.Sprintf("--- a/%s\n+++ b/%s\n", f.Filename, f.Filename)
		sb.WriteString(header)
		if f.Patch != "" {
			sb.WriteString(f.Patch)
			sb.WriteByte('\n')
		}
	}

	full := sb.String()
	if len(full) <= diffMaxBytes {
		return full
	}

	cut := diffMaxBytes
	for cut > 0 && full[cut]&0xC0 == 0x80 {
		cut--
	}
	omitted := len(full) - cut
	return full[:cut] + fmt.Sprintf("\n[diff truncated — %d chars omitted]\n", omitted)
}

// renderPrompt writes the Markdown review prompt to w.
func renderPrompt(w io.Writer, meta prMeta, diff string) {
	schema := string(reviewschema.Schema())

	fmt.Fprintf(w, "# PR Review Request\n\n")
	fmt.Fprintf(w, "**PR**: #%d — %s\n", meta.Number, meta.Title)
	fmt.Fprintf(w, "**Author**: @%s\n", meta.User.Login)
	fmt.Fprintf(w, "**Changed files**: %d\n\n", meta.ChangedFiles)
	fmt.Fprintf(w, "## Diff\n\n```diff\n%s\n```\n\n", diff)
	fmt.Fprintf(w, "## Review Schema\n\n```json\n%s\n```\n\n", schema)
	fmt.Fprintf(w, "## Review Rules\n\n")
	fmt.Fprintf(w, "- Each comment MUST have `suggestion` (non-empty) OR `skip_suggestion: true` with `reason` (non-empty)\n")
	fmt.Fprintf(w, "- Verify line numbers against the diff before setting `line`\n")
	fmt.Fprintf(w, "- `summary` must NOT contain suggestion fences\n")
	fmt.Fprintf(w, "- Use `start_line` + `line` for multi-line code suggestions\n")
}
