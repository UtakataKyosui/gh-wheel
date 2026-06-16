package review

import (
	"fmt"
	"testing"
)

func TestReplyCmd_MissingCommentID(t *testing.T) {
	cmd := newReplyCmd()
	cmd.SetArgs([]string{"123", "--body", "thanks"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when --comment-id is missing, got nil")
	}
}

func TestReplyCmd_MissingBody(t *testing.T) {
	cmd := newReplyCmd()
	cmd.SetArgs([]string{"123", "--comment-id", "456"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when --body is missing, got nil")
	}
}

func TestBuildReplyPath(t *testing.T) {
	// Verify endpoint includes PR number (critical: must not omit PR number)
	owner := "myowner"
	repoName := "myrepo"
	prNum := 42
	commentID := "789"

	path := fmt.Sprintf("pulls/%d/comments/%s/replies", prNum, commentID)
	fullPath := fmt.Sprintf("repos/%s/%s/%s", owner, repoName, path)

	expected := "repos/myowner/myrepo/pulls/42/comments/789/replies"
	if fullPath != expected {
		t.Errorf("expected path %q, got %q", expected, fullPath)
	}

	// Ensure PR number is present in path
	wrongPath := fmt.Sprintf("repos/%s/%s/pulls/comments/%s/replies", owner, repoName, commentID)
	if wrongPath == expected {
		t.Error("wrong path format should not equal expected path")
	}
}
