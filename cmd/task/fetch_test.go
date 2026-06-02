package task

import (
	"testing"
	"time"
)

func TestMergePRs_AuthorOnly(t *testing.T) {
	now := time.Now()
	items := []searchItem{
		{Number: 1, Title: "PR one", HTMLURL: "https://github.com/o/r/pull/1", State: "open", UpdatedAt: now, Draft: false},
	}
	prs := mergePRs(items, nil, true)

	if len(prs) != 1 {
		t.Fatalf("want 1 PR, got %d", len(prs))
	}
	if prs[0].Number != 1 {
		t.Errorf("PR.Number: want 1, got %d", prs[0].Number)
	}
	if len(prs[0].Categories) != 1 || prs[0].Categories[0] != "author" {
		t.Errorf("Categories: want [author], got %v", prs[0].Categories)
	}
}

func TestMergePRs_ReviewOnly(t *testing.T) {
	now := time.Now()
	items := []searchItem{
		{Number: 2, Title: "PR two", State: "open", UpdatedAt: now},
	}
	prs := mergePRs(nil, items, true)

	if len(prs) != 1 {
		t.Fatalf("want 1 PR, got %d", len(prs))
	}
	if len(prs[0].Categories) != 1 || prs[0].Categories[0] != "review-requested" {
		t.Errorf("Categories: want [review-requested], got %v", prs[0].Categories)
	}
}

func TestMergePRs_AuthorAndReview(t *testing.T) {
	now := time.Now()
	item := searchItem{Number: 3, Title: "shared", State: "open", UpdatedAt: now}
	prs := mergePRs([]searchItem{item}, []searchItem{item}, true)

	if len(prs) != 1 {
		t.Fatalf("dedup failed: want 1 PR, got %d", len(prs))
	}
	cats := prs[0].Categories
	if len(cats) != 2 {
		t.Errorf("want 2 categories, got %v", cats)
	}
}

func TestMergePRs_ExcludeDraft(t *testing.T) {
	now := time.Now()
	items := []searchItem{
		{Number: 4, State: "open", UpdatedAt: now, Draft: true},
		{Number: 5, State: "open", UpdatedAt: now, Draft: false},
	}
	prs := mergePRs(items, nil, false)

	if len(prs) != 1 {
		t.Fatalf("want 1 non-draft PR, got %d", len(prs))
	}
	if prs[0].Number != 5 {
		t.Errorf("expected PR #5, got #%d", prs[0].Number)
	}
}

func TestMergePRs_SortedByUpdatedAt(t *testing.T) {
	older := time.Now().Add(-1 * time.Hour)
	newer := time.Now()
	items := []searchItem{
		{Number: 10, State: "open", UpdatedAt: older},
		{Number: 11, State: "open", UpdatedAt: newer},
	}
	prs := mergePRs(items, nil, true)

	if len(prs) != 2 {
		t.Fatalf("want 2 PRs, got %d", len(prs))
	}
	if prs[0].Number != 11 {
		t.Errorf("want newer PR first, got #%d", prs[0].Number)
	}
}

func TestToIssues_Labels(t *testing.T) {
	now := time.Now()
	items := []searchItem{
		{
			Number:    20,
			Title:     "issue",
			State:     "open",
			UpdatedAt: now,
			Labels: []struct {
				Name string `json:"name"`
			}{{Name: "bug"}, {Name: "help wanted"}},
		},
	}
	issues := toIssues(items)

	if len(issues) != 1 {
		t.Fatalf("want 1 issue, got %d", len(issues))
	}
	if len(issues[0].Labels) != 2 {
		t.Errorf("want 2 labels, got %v", issues[0].Labels)
	}
	if issues[0].Labels[0] != "bug" {
		t.Errorf("want first label 'bug', got %q", issues[0].Labels[0])
	}
}

func TestStateQuery(t *testing.T) {
	cases := []struct {
		state string
		want  string
	}{
		{"open", "+is:open"},
		{"closed", "+is:closed"},
		{"all", ""},
		{"", "+is:open"},
	}
	for _, c := range cases {
		if got := stateQuery(c.state); got != c.want {
			t.Errorf("stateQuery(%q): want %q, got %q", c.state, c.want, got)
		}
	}
}
