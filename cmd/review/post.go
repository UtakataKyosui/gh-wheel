package review

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/UtakataKyosui/gh-wheel/internal/cliexit"
	"github.com/UtakataKyosui/gh-wheel/internal/ghclient"
	"github.com/spf13/cobra"
)

// reviewPostPayload is the JSON body for POST .../pulls/{N}/reviews.
type reviewPostPayload struct {
	Body     string                 `json:"body"`
	Event    string                 `json:"event"`
	Comments []reviewPayloadComment `json:"comments"`
}

// reviewPayloadComment is a single inline comment within a review payload.
// StartLine uses omitempty so it is excluded from JSON when zero.
type reviewPayloadComment struct {
	Path      string `json:"path"`
	Body      string `json:"body"`
	Side      string `json:"side"`
	Line      int    `json:"line"`
	StartLine int    `json:"start_line,omitempty"`
}

// newPostCmd builds the `gh wheel review post` subcommand.
func newPostCmd() *cobra.Command {
	var (
		flagFile        string
		flagMinComments int
		flagStrict      bool
		flagDryRun      bool
		flagFormat      string
		flagRepo        string
	)

	cmd := &cobra.Command{
		Use:   "post <PR> -f <file>",
		Short: "Post a structured review (JSON/YAML) to a GitHub PR",
		Long:  "Validate and post an AI-generated structured review file to a GitHub Pull Request.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagFile == "" {
				return cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs,
					fmt.Errorf("--file is required"))
			}

			var prNum int
			if _, err := fmt.Sscanf(args[0], "%d", &prNum); err != nil || prNum <= 0 {
				return cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs,
					fmt.Errorf("invalid PR number %q", args[0]))
			}

			doc, err := parseReviewFile(flagFile, flagFormat)
			if err != nil {
				return cliexit.NewGeneral(fmt.Errorf("parse review file: %w", err))
			}

			opts := validateOpts{
				MinComments: flagMinComments,
				Strict:      flagStrict,
				PRNum:       prNum,
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
				return cliexit.NewValidation(cliexit.ErrCodeValidation,
					fmt.Errorf("validation failed with %d error(s)", len(errs)),
					map[string]any{"errors": errs})
			}

			payload := buildReviewPayload(doc)

			if flagDryRun {
				out := struct {
					SchemaVersion string            `json:"schema_version"`
					Kind          string            `json:"kind"`
					Payload       reviewPostPayload `json:"payload"`
				}{
					SchemaVersion: "v1",
					Kind:          "review_post_preview",
					Payload:       payload,
				}
				data, err := json.MarshalIndent(out, "", "  ")
				if err != nil {
					return cliexit.NewGeneral(fmt.Errorf("marshal payload: %w", err))
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			c, err := ghclient.New(flagRepo)
			if err != nil {
				return err
			}

			htmlURL, err := postReview(c, prNum, payload)
			if err != nil {
				return cliexit.NewAPI(cliexit.ErrCodeAPI, fmt.Errorf("post review: %w", err))
			}

			fmt.Fprintf(cmd.OutOrStdout(), "✓ Review posted: %s\n", htmlURL)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flagFile, "file", "f", "", "Path to the review JSON/YAML file (required)")
	cmd.Flags().IntVar(&flagMinComments, "min-comments", 0, "Minimum comment count (0 = use default of 1)")
	cmd.Flags().BoolVar(&flagStrict, "strict", false, "Treat warnings as errors")
	cmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Print payload JSON without posting to GitHub")
	cmd.Flags().StringVar(&flagFormat, "format", "", "File format: yaml|json (default: auto-detect by extension)")
	cmd.Flags().StringVar(&flagRepo, "repo", "", "Repository (owner/name), defaults to current directory")

	return cmd
}

// buildReviewPayload converts a validated ReviewDoc into the GitHub API payload.
func buildReviewPayload(doc ReviewDoc) reviewPostPayload {
	event := strings.ToUpper(doc.Event)
	if event == "" {
		event = "COMMENT"
	}

	comments := make([]reviewPayloadComment, 0, len(doc.Comments))
	for _, c := range doc.Comments {
		body := c.Body

		// Append suggestion fence when a suggestion is provided.
		if c.Suggestion != "" {
			body += "\n```suggestion\n" + c.Suggestion + "\n```"
		}

		// Append skip-reason HTML comment when skip_suggestion is true and reason is set.
		if c.SkipSuggestion && c.Reason != "" {
			body += "\n<!-- skip-reason: " + c.Reason + " -->"
		}

		side := c.Side
		if side == "" {
			side = "RIGHT"
		}

		pc := reviewPayloadComment{
			Path: c.Path,
			Body: body,
			Side: side,
			Line: c.Line,
		}

		// Include StartLine only when it is positive and strictly less than Line.
		if c.StartLine > 0 && c.StartLine < c.Line {
			pc.StartLine = c.StartLine
		}

		comments = append(comments, pc)
	}

	return reviewPostPayload{
		Body:     doc.Summary,
		Event:    event,
		Comments: comments,
	}
}

// postReview sends the review payload to GitHub and returns the html_url of the created review.
func postReview(c *ghclient.Client, prNum int, payload reviewPostPayload) (string, error) {
	var resp struct {
		HTMLURL string `json:"html_url"`
	}
	path := fmt.Sprintf("pulls/%d/reviews", prNum)
	if err := c.RepoPost(path, payload, &resp); err != nil {
		return "", err
	}
	return resp.HTMLURL, nil
}
