package review

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/UtakataKyosui/gh-wheel/internal/ghclient"
)

type replyPayload struct {
	Body string `json:"body"`
}

type replyResponse struct {
	ID      int    `json:"id"`
	HTMLURL string `json:"html_url"`
}

func newReplyCmd() *cobra.Command {
	var commentID string
	var body string
	var flagRepo string

	cmd := &cobra.Command{
		Use:   "reply <PR>",
		Short: "Post a reply to a PR review comment",
		Long:  "Post a reply to a specific pull request review comment by comment ID.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if commentID == "" {
				return errors.New("--comment-id is required")
			}
			if body == "" {
				return errors.New("--body is required")
			}

			prNum, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid PR number %q: %w", args[0], err)
			}

			c, err := ghclient.New(flagRepo)
			if err != nil {
				return err
			}

			resp, err := postReply(c, prNum, commentID, body)
			if err != nil {
				return err
			}

			fmt.Printf("✓ Reply posted: %s\n", resp.HTMLURL)
			return nil
		},
	}

	cmd.Flags().StringVar(&commentID, "comment-id", "", "reply target comment ID (required)")
	cmd.Flags().StringVar(&body, "body", "", "reply text (required)")
	cmd.Flags().StringVarP(&flagRepo, "repo", "R", "", "repository (owner/name); defaults to cwd")

	return cmd
}

func postReply(c *ghclient.Client, prNum int, commentID, body string) (replyResponse, error) {
	path := fmt.Sprintf("pulls/%d/comments/%s/replies", prNum, commentID)
	payload := replyPayload{Body: body}
	var resp replyResponse
	if err := c.RepoPost(path, payload, &resp); err != nil {
		return replyResponse{}, err
	}
	return resp, nil
}
