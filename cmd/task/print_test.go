package task

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestPrintTable_NoPRs(t *testing.T) {
	var buf bytes.Buffer
	printTable(&buf, &TaskResult{
		Repository: "owner/repo",
		User:       "alice",
		PRs:        []PR{},
		Issues:     []Issue{},
	})
	out := buf.String()
	if !strings.Contains(out, "No results") {
		t.Errorf("expected 'No results', got: %q", out)
	}
}

func TestPrintTable_PR(t *testing.T) {
	now := time.Now()
	var buf bytes.Buffer
	printTable(&buf, &TaskResult{
		Repository: "owner/repo",
		User:       "alice",
		PRs: []PR{
			{
				Number:     42,
				Title:      "my feature",
				State:      "open",
				IsDraft:    false,
				UpdatedAt:  now,
				Categories: []string{"author"},
				Labels:     []string{},
				Reviews:    []Review{},
				ReviewComments: []ReviewComment{},
			},
		},
		Issues: []Issue{},
	})
	out := buf.String()
	if !strings.Contains(out, "#42") {
		t.Errorf("expected '#42' in output, got: %q", out)
	}
	if !strings.Contains(out, "my feature") {
		t.Errorf("expected title in output, got: %q", out)
	}
	if !strings.Contains(out, "[A]") {
		t.Errorf("expected '[A]' badge for author, got: %q", out)
	}
}

func TestPrintTable_Draft(t *testing.T) {
	var buf bytes.Buffer
	printTable(&buf, &TaskResult{
		PRs: []PR{
			{
				Number:     99,
				Title:      "draft PR",
				State:      "open",
				IsDraft:    true,
				Categories: []string{"author"},
				Labels:     []string{},
				Reviews:    []Review{},
				ReviewComments: []ReviewComment{},
			},
		},
		Issues: []Issue{},
	})
	out := buf.String()
	if !strings.Contains(out, "draft") {
		t.Errorf("expected 'draft' indicator, got: %q", out)
	}
}

func TestPrintTable_CategoryBadges(t *testing.T) {
	cases := []struct {
		cats []string
		want string
	}{
		{[]string{"author"}, "[A]"},
		{[]string{"review-requested"}, "[R]"},
		{[]string{"author", "review-requested"}, "[AR]"},
	}
	for _, tc := range cases {
		var buf bytes.Buffer
		printTable(&buf, &TaskResult{
			PRs: []PR{
				{Number: 1, Categories: tc.cats, Labels: []string{}, Reviews: []Review{}, ReviewComments: []ReviewComment{}},
			},
			Issues: []Issue{},
		})
		out := buf.String()
		if !strings.Contains(out, tc.want) {
			t.Errorf("cats=%v: want %q in output, got: %q", tc.cats, tc.want, out)
		}
	}
}

func TestPrintTable_Issue(t *testing.T) {
	var buf bytes.Buffer
	printTable(&buf, &TaskResult{
		PRs: []PR{},
		Issues: []Issue{
			{Number: 7, Title: "bug report", State: "open", Labels: []string{}},
		},
	})
	out := buf.String()
	if !strings.Contains(out, "#7") {
		t.Errorf("expected '#7' in output, got: %q", out)
	}
	if !strings.Contains(out, "bug report") {
		t.Errorf("expected issue title in output, got: %q", out)
	}
}
