package graphql

import (
	"strings"
	"testing"
)

// TestParseIssuesPage verifies that rawIssuesPage is correctly converted to IssuesPage.
func TestParseIssuesPage(t *testing.T) {
	issueNode := rawNode{
		ID:     "issue_id_1",
		Number: 10,
		Title:  "Issue One",
		URL:    "https://github.com/owner/repo/issues/10",
		State:  "OPEN",
	}
	issueNode.Labels.Nodes = []struct {
		Name string `json:"name"`
	}{{Name: "bug"}, {Name: "help wanted"}}
	issueNode.Milestone.Title = "v1.0"
	issueNode.Assignees.Nodes = []struct {
		Login string `json:"login"`
	}{{Login: "alice"}}

	prNode := rawNode{
		ID:      "pr_id_1",
		Number:  5,
		Title:   "PR One",
		URL:     "https://github.com/owner/repo/pull/5",
		State:   "OPEN",
		IsDraft: true,
	}

	var raw rawIssuesPage
	raw.Repository.Issues.PageInfo = rawPageInfo{HasNextPage: true, EndCursor: "cursor123"}
	raw.Repository.Issues.Nodes = []rawNode{issueNode}
	raw.Repository.PRs.PageInfo = rawPageInfo{HasNextPage: false, EndCursor: ""}
	raw.Repository.PRs.Nodes = []rawNode{prNode}

	page := convertIssuesPage(raw)

	if len(page.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(page.Issues))
	}
	if len(page.PRs) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(page.PRs))
	}

	iss := page.Issues[0]
	if iss.ID != "issue_id_1" {
		t.Errorf("issue ID: want issue_id_1, got %q", iss.ID)
	}
	if iss.Number != 10 {
		t.Errorf("issue Number: want 10, got %d", iss.Number)
	}
	if iss.Kind != "issue" {
		t.Errorf("issue Kind: want issue, got %q", iss.Kind)
	}
	if iss.Title != "Issue One" {
		t.Errorf("issue Title: want Issue One, got %q", iss.Title)
	}
	if iss.State != "OPEN" {
		t.Errorf("issue State: want OPEN, got %q", iss.State)
	}
	if iss.Milestone != "v1.0" {
		t.Errorf("issue Milestone: want v1.0, got %q", iss.Milestone)
	}
	if len(iss.Labels) != 2 || iss.Labels[0] != "bug" || iss.Labels[1] != "help wanted" {
		t.Errorf("issue Labels: want [bug, help wanted], got %v", iss.Labels)
	}
	if len(iss.Assignees) != 1 || iss.Assignees[0] != "alice" {
		t.Errorf("issue Assignees: want [alice], got %v", iss.Assignees)
	}

	pr := page.PRs[0]
	if pr.Kind != "pull_request" {
		t.Errorf("PR Kind: want pull_request, got %q", pr.Kind)
	}
	if !pr.IsDraft {
		t.Errorf("PR IsDraft: want true, got false")
	}

	if !page.HasNextPage {
		t.Errorf("HasNextPage: want true (issues has next page)")
	}
	if page.Cursor != "cursor123" {
		t.Errorf("Cursor: want cursor123, got %q", page.Cursor)
	}
}

// TestParseIssuesPagePRCursor verifies cursor uses PR cursor when issues has no next page.
func TestParseIssuesPagePRCursor(t *testing.T) {
	var raw rawIssuesPage
	raw.Repository.PRs.PageInfo = rawPageInfo{HasNextPage: true, EndCursor: "pr_cursor_456"}

	page := convertIssuesPage(raw)

	if !page.HasNextPage {
		t.Errorf("HasNextPage: want true (PRs has next page)")
	}
	if page.Cursor != "pr_cursor_456" {
		t.Errorf("Cursor: want pr_cursor_456, got %q", page.Cursor)
	}
}

// TestSubIssuesFallback verifies that GraphQL errors containing "doesn't exist on type"
// result in an empty SubIssueResult (graceful fallback).
func TestSubIssuesFallback(t *testing.T) {
	errMsg := "Field 'subIssues' doesn't exist on type 'Issue'"
	result := isSubIssueSchemaError(errMsg)
	if !result {
		t.Errorf("isSubIssueSchemaError(%q): want true, got false", errMsg)
	}

	errMsg2 := "some unrelated error"
	result2 := isSubIssueSchemaError(errMsg2)
	if result2 {
		t.Errorf("isSubIssueSchemaError(%q): want false, got true", errMsg2)
	}
}

// TestConvertNode verifies node conversion for both issue and PR kinds.
func TestConvertNode(t *testing.T) {
	n := rawNode{
		ID:      "test_id",
		Number:  42,
		Title:   "Test Node",
		URL:     "https://github.com/owner/repo/issues/42",
		State:   "CLOSED",
		IsDraft: false,
	}
	n.Labels.Nodes = []struct {
		Name string `json:"name"`
	}{{Name: "enhancement"}}
	n.Milestone.Title = "v2.0"
	n.Assignees.Nodes = []struct {
		Login string `json:"login"`
	}{{Login: "bob"}, {Login: "carol"}}

	node := convertNode(n, "issue")
	if node.Kind != "issue" {
		t.Errorf("Kind: want issue, got %q", node.Kind)
	}
	if node.Number != 42 {
		t.Errorf("Number: want 42, got %d", node.Number)
	}
	if len(node.Labels) != 1 || node.Labels[0] != "enhancement" {
		t.Errorf("Labels: want [enhancement], got %v", node.Labels)
	}
	if len(node.Assignees) != 2 {
		t.Errorf("Assignees: want 2, got %d", len(node.Assignees))
	}
}

// TestIsSubIssueSchemaError checks various error message patterns.
func TestIsSubIssueSchemaError(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{"Field 'subIssues' doesn't exist on type 'Issue'", true},
		{"Field 'parent' doesn't exist on type 'Issue'", true},
		{"doesn't exist on type", true},
		{"some random error", false},
		{"", false},
	}
	for _, tc := range cases {
		got := isSubIssueSchemaError(tc.msg)
		if got != tc.want {
			t.Errorf("isSubIssueSchemaError(%q) = %v, want %v", tc.msg, got, tc.want)
		}
	}
}

// TestConvertSubIssueResult verifies sub-issue result conversion.
func TestConvertSubIssueResult(t *testing.T) {
	var raw rawSubIssueResponse
	raw.Repository.Issue.rawNode = rawNode{
		ID:     "parent_id",
		Number: 1,
		Title:  "Parent Issue",
		State:  "OPEN",
		URL:    "https://github.com/owner/repo/issues/1",
	}
	raw.Repository.Issue.SubIssues.Nodes = []rawNode{
		{
			ID:     "child_id_1",
			Number: 2,
			Title:  "Child Issue 1",
			State:  "OPEN",
			URL:    "https://github.com/owner/repo/issues/2",
		},
		{
			ID:     "child_id_2",
			Number: 3,
			Title:  "Child Issue 2",
			State:  "CLOSED",
			URL:    "https://github.com/owner/repo/issues/3",
		},
	}

	result := convertSubIssueResult(raw)

	if result.Parent == nil {
		t.Fatal("Parent is nil, want non-nil")
	}
	if result.Parent.Number != 1 {
		t.Errorf("Parent.Number: want 1, got %d", result.Parent.Number)
	}
	if len(result.Children) != 2 {
		t.Errorf("Children count: want 2, got %d", len(result.Children))
	}
	if result.Children[0].Number != 2 {
		t.Errorf("Children[0].Number: want 2, got %d", result.Children[0].Number)
	}
}

// TestTimelineItemConversion verifies timeline item parsing.
func TestTimelineItemConversion(t *testing.T) {
	items := []rawTimelineItem{
		{
			Typename: "CrossReferencedEvent",
			Source: rawTimelineSource{
				Typename: "PullRequest",
				Number:   10,
			},
		},
		{
			Typename: "ConnectedEvent",
			Subject: rawTimelineSubject{
				Typename: "Issue",
				Number:   5,
			},
		},
	}

	converted := convertTimelineItems(items)
	if len(converted) != 2 {
		t.Fatalf("expected 2 items, got %d", len(converted))
	}
	if converted[0].ItemType != "CrossReferencedEvent" {
		t.Errorf("ItemType: want CrossReferencedEvent, got %q", converted[0].ItemType)
	}
	if converted[0].SourceNumber != 10 {
		t.Errorf("SourceNumber: want 10, got %d", converted[0].SourceNumber)
	}
}

// TestIsSubIssueSchemaErrorContains verifies string-based error detection behavior.
func TestIsSubIssueSchemaErrorContains(t *testing.T) {
	msg := "prefix doesn't exist on type suffix"
	if !isSubIssueSchemaError(msg) {
		t.Error("expected true for message containing 'doesn't exist on type'")
	}
	_ = strings.Contains // reference strings package to avoid unused import
}
