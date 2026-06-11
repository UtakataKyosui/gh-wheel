package task

import (
	"testing"
	"time"
)

func TestRoleIcon(t *testing.T) {
	cases := []struct {
		role string
		want string
	}{
		{"both", "✏️👁"},
		{"author", "✏️  "},
		{"review-requested", " 👁 "},
		{"", "    "},
	}
	for _, tc := range cases {
		got := roleIcon(tc.role)
		if got != tc.want {
			t.Errorf("roleIcon(%q) = %q, want %q", tc.role, got, tc.want)
		}
	}
}

func TestStateLabel(t *testing.T) {
	cases := []struct {
		state   string
		isDraft bool
		want    string
	}{
		{"open", false, "open  "},
		{"closed", false, "closed"},
		{"open", true, "draft "},
		{"merged", false, "merged"},
	}
	for _, tc := range cases {
		got := stateLabel(tc.state, tc.isDraft)
		if got != tc.want {
			t.Errorf("stateLabel(%q, %v) = %q, want %q", tc.state, tc.isDraft, got, tc.want)
		}
	}
}

func TestBuildTUIItems_PR(t *testing.T) {
	result := &TaskResult{
		Repository: "owner/repo",
		PRs: []PR{
			{Number: 42, Title: "feat: add TUI", URL: "https://github.com/o/r/pull/42",
				State: "open", Categories: []string{"author"},
				Reviews: []Review{}, ReviewComments: []ReviewComment{}},
		},
		Issues: []Issue{},
	}
	items := buildTUIItems(result, false)
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %d", len(items))
	}
	it, ok := items[0].(tuiItem)
	if !ok {
		t.Fatal("item is not tuiItem")
	}
	if it.number != 42 {
		t.Errorf("number: want 42, got %d", it.number)
	}
	if it.role != "author" {
		t.Errorf("role: want author, got %q", it.role)
	}
	if it.isIssue {
		t.Error("isIssue should be false for a PR")
	}
}

func TestBuildTUIItems_RoleBoth(t *testing.T) {
	result := &TaskResult{
		PRs: []PR{
			{Number: 1, Categories: []string{"author", "review-requested"},
				Reviews: []Review{}, ReviewComments: []ReviewComment{}},
		},
		Issues: []Issue{},
	}
	items := buildTUIItems(result, false)
	it := items[0].(tuiItem)
	if it.role != "both" {
		t.Errorf("want role=both, got %q", it.role)
	}
}

func TestBuildTUIItems_Issue(t *testing.T) {
	result := &TaskResult{
		PRs: []PR{},
		Issues: []Issue{
			{Number: 7, Title: "bug", URL: "https://github.com/o/r/issues/7", State: "open"},
		},
	}
	items := buildTUIItems(result, false)
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %d", len(items))
	}
	it := items[0].(tuiItem)
	if !it.isIssue {
		t.Error("isIssue should be true for an Issue")
	}
	if it.role != "" {
		t.Errorf("issues should have no role, got %q", it.role)
	}
}

func TestBuildTUIItems_WithReviews(t *testing.T) {
	now := time.Now()
	result := &TaskResult{
		PRs: []PR{
			{
				Number:     10,
				Categories: []string{"review-requested"},
				Reviews: []Review{
					{Author: "alice", UserType: "User", State: "APPROVED", SubmittedAt: now},
				},
				ReviewComments: []ReviewComment{},
			},
		},
		Issues: []Issue{},
	}
	items := buildTUIItems(result, true)
	it := items[0].(tuiItem)
	if it.humanReview != "✅" {
		t.Errorf("humanReview: want ✅, got %q", it.humanReview)
	}
	if it.aiReview != "⏳🤖" {
		t.Errorf("aiReview: want ⏳🤖, got %q", it.aiReview)
	}
}

func TestBuildTUIItems_FilterValue(t *testing.T) {
	result := &TaskResult{
		PRs: []PR{
			{Number: 1, Title: "feat: my feature", Categories: []string{"author"},
				Reviews: []Review{}, ReviewComments: []ReviewComment{}},
		},
		Issues: []Issue{},
	}
	items := buildTUIItems(result, false)
	it := items[0].(tuiItem)
	if it.FilterValue() != "feat: my feature" {
		t.Errorf("FilterValue: want %q, got %q", "feat: my feature", it.FilterValue())
	}
}
