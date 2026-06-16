package review

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/UtakataKyosui/gh-wheel/internal/ghclient"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// ReviewDoc is the top-level structure for an AI-generated review file.
type ReviewDoc struct {
	Event    string          `json:"event"    yaml:"event"`
	Summary  string          `json:"summary"  yaml:"summary"`
	Comments []ReviewComment `json:"comments" yaml:"comments"`
}

// ReviewComment is a single review comment entry.
type ReviewComment struct {
	Path           string `json:"path"            yaml:"path"`
	Body           string `json:"body"            yaml:"body"`
	Suggestion     string `json:"suggestion"      yaml:"suggestion"`
	Reason         string `json:"reason"          yaml:"reason"`
	Side           string `json:"side"            yaml:"side"`
	Line           int    `json:"line"            yaml:"line"`
	StartLine      int    `json:"start_line"      yaml:"start_line"`
	SkipSuggestion bool   `json:"skip_suggestion" yaml:"skip_suggestion"`
}

// validateOpts holds runtime options for ValidateDoc.
type validateOpts struct {
	MinComments int
	Strict      bool
	PRNum       int
	CurrentUser string
	// PRAuthor is populated by callers that already fetched PR metadata.
	PRAuthor string
}

// writeFile is a small helper used by tests to create temp files.
func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}

// newValidateCmd builds the `gh wheel review validate` subcommand.
func newValidateCmd() *cobra.Command {
	var (
		flagFile        string
		flagPR          int
		flagMinComments int
		flagStrict      bool
		flagFormat      string
		flagRepo        string
	)

	cmd := &cobra.Command{
		Use:   "validate -f <file>",
		Short: "Validate an AI-generated review JSON/YAML file before posting",
		Long:  "Gate-keeper that validates AI-generated review JSON/YAML before posting to GitHub.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagFile == "" {
				return fmt.Errorf("--file is required")
			}

			doc, err := parseReviewFile(flagFile, flagFormat)
			if err != nil {
				return fmt.Errorf("parse review file: %w", err)
			}

			opts := validateOpts{
				MinComments: flagMinComments,
				Strict:      flagStrict,
				PRNum:       flagPR,
			}

			// Fetch dynamic thresholds / PR metadata when --pr is given.
			if flagPR > 0 {
				c, err := ghclient.New(flagRepo)
				if err != nil {
					return fmt.Errorf("ghclient: %w", err)
				}

				// Fetch changed_files count for dynamic min-comment threshold.
				if flagMinComments == 0 {
					var prResp struct {
						ChangedFiles int    `json:"changed_files"`
						User         struct{ Login string } `json:"user"`
					}
					path := fmt.Sprintf("pulls/%d", flagPR)
					if err := c.RepoGet(path, &prResp); err != nil {
						return fmt.Errorf("fetch PR: %w", err)
					}
					switch {
					case prResp.ChangedFiles <= 4:
						opts.MinComments = 1
					case prResp.ChangedFiles <= 20:
						opts.MinComments = 3
					default:
						opts.MinComments = 5
					}
					opts.PRAuthor = prResp.User.Login
				}

				// Always fetch current user for self-approve guard.
				me, err := c.CurrentUser()
				if err != nil {
					return fmt.Errorf("current user: %w", err)
				}
				opts.CurrentUser = me
			}

			if opts.MinComments == 0 {
				opts.MinComments = 1
			}

			warnings, errs := ValidateDoc(doc, opts)

			for _, w := range warnings {
				fmt.Fprintf(cmd.ErrOrStderr(), "WARNING: %s\n", w)
			}
			for _, e := range errs {
				fmt.Fprintf(cmd.ErrOrStderr(), "ERROR:   %s\n", e)
			}

			if len(errs) > 0 {
				return fmt.Errorf("validation failed with %d error(s)", len(errs))
			}
			fmt.Fprintln(cmd.OutOrStdout(), "OK: review file is valid")
			return nil
		},
	}

	cmd.Flags().StringVarP(&flagFile, "file", "f", "", "Path to the review JSON/YAML file (required)")
	cmd.Flags().IntVar(&flagPR, "pr", 0, "PR number (used to fetch changed_files for dynamic threshold)")
	cmd.Flags().IntVar(&flagMinComments, "min-comments", 0, "Override minimum comment count (0 = use dynamic)")
	cmd.Flags().BoolVar(&flagStrict, "strict", false, "Treat warnings as errors")
	cmd.Flags().StringVar(&flagFormat, "format", "", "File format: yaml|json (default: auto-detect by extension)")
	cmd.Flags().StringVar(&flagRepo, "repo", "", "Repository (owner/name), defaults to current directory")

	return cmd
}

// parseReviewFile reads and unmarshals a review file.
// format overrides auto-detection when non-empty.
func parseReviewFile(path, format string) (ReviewDoc, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ReviewDoc{}, fmt.Errorf("read file: %w", err)
	}

	if format == "" {
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".json":
			format = "json"
		case ".yaml", ".yml":
			format = "yaml"
		default:
			return ReviewDoc{}, fmt.Errorf("unknown file extension %q; use --format yaml|json", ext)
		}
	}

	var doc ReviewDoc
	switch strings.ToLower(format) {
	case "json":
		if err := json.Unmarshal(data, &doc); err != nil {
			return ReviewDoc{}, fmt.Errorf("parse JSON: %w", err)
		}
	case "yaml":
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return ReviewDoc{}, fmt.Errorf("parse YAML: %w", err)
		}
	default:
		return ReviewDoc{}, fmt.Errorf("unknown format %q; must be yaml or json", format)
	}
	return doc, nil
}

// validEvents is the set of allowed event values.
var validEvents = map[string]bool{
	"COMMENT":         true,
	"REQUEST_CHANGES": true,
	"APPROVE":         true,
}

// ValidateDoc runs all validation rules and returns warnings and errors.
// It is exported so post.go (and tests) can reuse it.
func ValidateDoc(doc ReviewDoc, opts validateOpts) (warnings []string, errs []string) {
	// Rule 2: Required fields non-empty.
	if doc.Event == "" {
		errs = append(errs, "event is required")
	}
	if doc.Summary == "" {
		errs = append(errs, "summary is required")
	}
	if len(doc.Comments) == 0 {
		errs = append(errs, "comments must not be empty")
	}

	// Rule 3: event must be one of the valid values.
	if doc.Event != "" && !validEvents[doc.Event] {
		errs = append(errs, fmt.Sprintf("event %q is invalid; must be one of COMMENT, REQUEST_CHANGES, APPROVE", doc.Event))
	}

	// Rule 4: summary must not contain suggestion fence.
	if strings.Contains(doc.Summary, "```suggestion") {
		errs = append(errs, "suggestion fence (```suggestion) must not appear in summary; put suggestions in comments only")
	}

	// Rule 5: minimum comment count.
	if opts.MinComments > 0 && len(doc.Comments) < opts.MinComments {
		errs = append(errs, fmt.Sprintf("at least %d comment(s) required, got %d", opts.MinComments, len(doc.Comments)))
	}

	// Rules 6, 7, 8, 9: per-comment validation.
	skipCount := 0
	for i, c := range doc.Comments {
		idx := i + 1

		if c.SkipSuggestion {
			skipCount++
			// Rule 7: skip_suggestion==true requires reason.
			if c.Reason == "" {
				errs = append(errs, fmt.Sprintf("comment[%d] path=%q: skip_suggestion is true but reason is empty", idx, c.Path))
			}
		} else if c.Suggestion == "" {
			// Rule 6: must have suggestion OR (skip_suggestion + reason).
			errs = append(errs, fmt.Sprintf("comment[%d] path=%q: must have a suggestion or set skip_suggestion=true with a reason", idx, c.Path))
		} else {
			// Rule 9: validate suggestion fence structure.
			if fenceErrs := validateSuggestionFence(c.Suggestion, idx, c.Path); len(fenceErrs) > 0 {
				errs = append(errs, fenceErrs...)
			}
		}
	}

	// Rule 8: skip ratio > 50%.
	if len(doc.Comments) > 0 {
		ratio := float64(skipCount) / float64(len(doc.Comments))
		if ratio > 0.5 {
			msg := fmt.Sprintf("skip ratio %.0f%% exceeds 50%%; consider providing more suggestions", ratio*100)
			if opts.Strict {
				errs = append(errs, msg)
			} else {
				warnings = append(warnings, msg)
			}
		}
	}

	// Rule 10: self-approve guard.
	if doc.Event == "APPROVE" && opts.PRAuthor != "" && opts.CurrentUser != "" {
		if opts.PRAuthor == opts.CurrentUser {
			errs = append(errs, fmt.Sprintf("self-approve detected: PR author and reviewer are both %q", opts.CurrentUser))
		}
	}

	return warnings, errs
}

// validateSuggestionFence checks language tag and fence closure for a suggestion string.
func validateSuggestionFence(suggestion string, commentIdx int, path string) []string {
	var errs []string

	lines := strings.Split(suggestion, "\n")
	if len(lines) == 0 {
		return errs
	}

	// Find the opening fence line (first line starting with ```).
	openingLine := strings.TrimSpace(lines[0])
	if strings.HasPrefix(openingLine, "```") {
		lang := strings.TrimPrefix(openingLine, "```")
		lang = strings.TrimSpace(lang)
		if lang == "" {
			errs = append(errs, fmt.Sprintf("comment[%d] path=%q: suggestion opening fence is missing a language tag (e.g. ```go)", commentIdx, path))
		}
	}

	// Check for unclosed fence: count occurrences of ``` sequences.
	// An even count means the fence is balanced (opened and closed).
	fenceCount := strings.Count(suggestion, "```")
	if fenceCount%2 != 0 {
		errs = append(errs, fmt.Sprintf("comment[%d] path=%q: suggestion has an unclosed fence (odd number of ``` sequences)", commentIdx, path))
	}

	return errs
}
