package task

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTaskResult_JSONRoundtrip(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	orig := TaskResult{
		Repository: "owner/repo",
		User:       "alice",
		FetchedAt:  now,
		PRs: []PR{
			{
				Number:         1,
				Title:          "my PR",
				URL:            "https://github.com/owner/repo/pull/1",
				Author:         "alice",
				State:          "open",
				IsDraft:        false,
				UpdatedAt:      now,
				Labels:         []string{"bug"},
				Categories:     []string{"author"},
				Body:           "body text",
				HeadRef:        "feat/foo",
				Reviews:        []Review{},
				ReviewComments: []ReviewComment{},
			},
		},
		Issues: []Issue{
			{
				Number:    2,
				Title:     "my issue",
				URL:       "https://github.com/owner/repo/issues/2",
				Author:    "alice",
				State:     "open",
				UpdatedAt: now,
				Labels:    []string{},
				Body:      "",
			},
		},
	}

	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got TaskResult
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Repository != orig.Repository {
		t.Errorf("Repository: want %q, got %q", orig.Repository, got.Repository)
	}
	if got.User != orig.User {
		t.Errorf("User: want %q, got %q", orig.User, got.User)
	}
	if len(got.PRs) != 1 {
		t.Fatalf("PRs length: want 1, got %d", len(got.PRs))
	}
	pr := got.PRs[0]
	if pr.Number != 1 {
		t.Errorf("PR.Number: want 1, got %d", pr.Number)
	}
	if pr.HeadRef != "feat/foo" {
		t.Errorf("PR.HeadRef: want %q, got %q", "feat/foo", pr.HeadRef)
	}
	if len(got.Issues) != 1 {
		t.Fatalf("Issues length: want 1, got %d", len(got.Issues))
	}
}

func TestPR_JSONFields(t *testing.T) {
	pr := PR{
		Number:         42,
		Categories:     []string{"author", "review-requested"},
		Labels:         []string{},
		Reviews:        []Review{},
		ReviewComments: []ReviewComment{},
	}

	b, err := json.Marshal(pr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}

	for _, key := range []string{"number", "title", "url", "author", "state", "isDraft",
		"updatedAt", "labels", "categories", "body", "headRef", "reviews", "reviewComments"} {
		if _, ok := m[key]; !ok {
			t.Errorf("JSON key %q missing", key)
		}
	}
}
