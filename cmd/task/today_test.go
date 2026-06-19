package task

import (
	"testing"
	"time"
)

var (
	baseT      = time.Date(2026, 6, 19, 9, 0, 0, 0, time.UTC)
	reviewT    = baseT.Add(1 * time.Hour)
	afterRevT  = baseT.Add(2 * time.Hour)
	beforeRevT = baseT.Add(-1 * time.Hour)
)

func humanReview(author, state string, at time.Time) Review {
	return Review{Author: author, UserType: "User", State: state, SubmittedAt: at}
}

func TestClassifyHumanReview(t *testing.T) {
	tests := []struct {
		name           string
		reviews        []Review
		latestCommitAt time.Time
		want           humanReviewClass
	}{
		{
			name:           "no reviews",
			reviews:        nil,
			latestCommitAt: baseT,
			want:           reviewNone,
		},
		{
			name:           "approved is sticky across later commit",
			reviews:        []Review{humanReview("alice", "APPROVED", reviewT)},
			latestCommitAt: afterRevT, // commit after the approval must NOT demote it
			want:           reviewApproved,
		},
		{
			name:           "changes requested, not addressed",
			reviews:        []Review{humanReview("alice", "CHANGES_REQUESTED", reviewT)},
			latestCommitAt: beforeRevT,
			want:           reviewChangesRequested,
		},
		{
			name:           "changes requested then addressed by newer commit",
			reviews:        []Review{humanReview("alice", "CHANGES_REQUESTED", reviewT)},
			latestCommitAt: afterRevT,
			want:           reviewAddressed,
		},
		{
			name:           "only comments",
			reviews:        []Review{humanReview("alice", "COMMENTED", reviewT)},
			latestCommitAt: baseT,
			want:           reviewCommented,
		},
		{
			name: "AI bot approval is ignored",
			reviews: []Review{
				{Author: "copilot[bot]", UserType: "Bot", State: "APPROVED", SubmittedAt: reviewT},
			},
			latestCommitAt: baseT,
			want:           reviewNone,
		},
		{
			name: "changes requested wins over approval from another reviewer",
			reviews: []Review{
				humanReview("alice", "APPROVED", reviewT),
				humanReview("bob", "CHANGES_REQUESTED", reviewT),
			},
			latestCommitAt: beforeRevT,
			want:           reviewChangesRequested,
		},
		{
			name: "latest review per author wins",
			reviews: []Review{
				humanReview("alice", "CHANGES_REQUESTED", beforeRevT),
				humanReview("alice", "APPROVED", reviewT),
			},
			latestCommitAt: baseT,
			want:           reviewApproved,
		},
		{
			name: "later comment does not clear changes requested",
			reviews: []Review{
				humanReview("alice", "CHANGES_REQUESTED", baseT),
				humanReview("alice", "COMMENTED", reviewT),
			},
			latestCommitAt: beforeRevT,
			want:           reviewChangesRequested,
		},
		{
			name: "later comment does not clear approval",
			reviews: []Review{
				humanReview("alice", "APPROVED", baseT),
				humanReview("alice", "COMMENTED", reviewT),
			},
			latestCommitAt: beforeRevT,
			want:           reviewApproved,
		},
		{
			name: "dismissed review carries no standing decision",
			reviews: []Review{
				humanReview("alice", "DISMISSED", reviewT),
			},
			latestCommitAt: baseT,
			want:           reviewNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyHumanReview(tt.reviews, tt.latestCommitAt)
			if got != tt.want {
				t.Errorf("classifyHumanReview() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestAlreadyReviewedByYou(t *testing.T) {
	tests := []struct {
		name string
		pr   PR
		want bool
	}{
		{
			name: "reviewed after latest commit",
			pr: PR{
				LatestCommitAt: baseT,
				Reviews:        []Review{humanReview("me", "COMMENTED", afterRevT)},
			},
			want: true,
		},
		{
			name: "reviewed before latest commit (author pushed since)",
			pr: PR{
				LatestCommitAt: afterRevT,
				Reviews:        []Review{humanReview("me", "COMMENTED", baseT)},
			},
			want: false,
		},
		{
			name: "someone else reviewed",
			pr: PR{
				LatestCommitAt: baseT,
				Reviews:        []Review{humanReview("alice", "APPROVED", afterRevT)},
			},
			want: false,
		},
		{
			name: "no commit data, any review by you counts",
			pr: PR{
				Reviews: []Review{humanReview("me", "COMMENTED", baseT)},
			},
			want: true,
		},
		{
			name: "your dismissed review does not count as reviewed",
			pr: PR{
				LatestCommitAt: baseT,
				Reviews:        []Review{humanReview("me", "DISMISSED", afterRevT)},
			},
			want: false,
		},
		{
			name: "your pending draft review does not count as reviewed",
			pr: PR{
				LatestCommitAt: baseT,
				Reviews:        []Review{humanReview("me", "PENDING", afterRevT)},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := alreadyReviewedByYou(tt.pr, "me"); got != tt.want {
				t.Errorf("alreadyReviewedByYou() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildPlanItems_ClassificationAndOrder(t *testing.T) {
	cfg := effortConfig{review: 20, merge: 10, prFix: 45, issue: 90}
	tr := &TaskResult{
		PRs: []PR{
			{Number: 11, Title: "fix me", Categories: []string{"author"},
				Reviews: []Review{humanReview("alice", "CHANGES_REQUESTED", reviewT)}, LatestCommitAt: beforeRevT},
			{Number: 10, Title: "merge me", Categories: []string{"author"},
				Reviews: []Review{humanReview("alice", "APPROVED", reviewT)}, LatestCommitAt: baseT},
			{Number: 12, Title: "review me", Categories: []string{"review-requested"}},
			{Number: 13, Title: "mine approved + review-requested", Categories: []string{"author", "review-requested"},
				Reviews: []Review{humanReview("alice", "APPROVED", reviewT)}, LatestCommitAt: baseT},
			{Number: 14, Title: "already reviewed", Categories: []string{"review-requested"},
				Reviews: []Review{humanReview("me", "APPROVED", afterRevT)}, LatestCommitAt: baseT},
			{Number: 15, Title: "my addressed pr", Categories: []string{"author"},
				Reviews: []Review{humanReview("alice", "CHANGES_REQUESTED", reviewT)}, LatestCommitAt: afterRevT},
		},
		Issues: []Issue{
			{Number: 20, Title: "in progress"},
		},
	}

	items := buildPlanItems(tr, "me", cfg)

	// #14 dropped (already reviewed), #15 dropped (addressed, awaiting re-review).
	gotTypes := make([]string, len(items))
	gotNums := make([]int, len(items))
	for i, it := range items {
		gotTypes[i] = it.Type
		gotNums[i] = it.Number
	}

	wantTypes := []string{typeReview, typePRMerge, typePRMerge, typePRFix, typeInProgress}
	if len(gotTypes) != len(wantTypes) {
		t.Fatalf("got %d items %v (nums %v), want %d %v", len(gotTypes), gotTypes, gotNums, len(wantTypes), wantTypes)
	}
	for i := range wantTypes {
		if gotTypes[i] != wantTypes[i] {
			t.Errorf("item[%d] type = %q, want %q (nums %v)", i, gotTypes[i], wantTypes[i], gotNums)
		}
	}

	// Dedup: #13 must appear exactly once and as pr-merge, never review.
	count13 := 0
	for _, it := range items {
		if it.Number == 13 {
			count13++
			if it.Type != typePRMerge {
				t.Errorf("#13 type = %q, want pr-merge", it.Type)
			}
		}
		if it.Number == 14 || it.Number == 15 {
			t.Errorf("#%d should be excluded, got type %q", it.Number, it.Type)
		}
	}
	if count13 != 1 {
		t.Errorf("#13 appeared %d times, want 1", count13)
	}

	// Estimates per category.
	for _, it := range items {
		switch it.Type {
		case typeReview:
			if it.EstimateMinutes != 20 {
				t.Errorf("review estimate = %d, want 20", it.EstimateMinutes)
			}
		case typePRMerge:
			if it.EstimateMinutes != 10 {
				t.Errorf("merge estimate = %d, want 10", it.EstimateMinutes)
			}
		case typePRFix:
			if it.EstimateMinutes != 45 {
				t.Errorf("fix estimate = %d, want 45", it.EstimateMinutes)
			}
		case typeInProgress:
			if it.EstimateMinutes != 90 {
				t.Errorf("in-progress estimate = %d, want 90", it.EstimateMinutes)
			}
		}
	}
}

func TestCandidateItems(t *testing.T) {
	cands := []candidate{
		{Number: 5, Title: "easy", Labels: []string{"good first issue"}, Score: 2},
		{Number: 6, Title: "plain", Score: 0},
	}
	items := candidateItems(cands, 90)
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Type != typeNew || items[0].Score != 2 || items[0].EstimateMinutes != 90 {
		t.Errorf("item0 = %+v", items[0])
	}
	if items[0].Reason != "approachable issue (score 2)" {
		t.Errorf("item0 reason = %q", items[0].Reason)
	}
	if items[1].Reason != "approachable issue" {
		t.Errorf("item1 reason = %q", items[1].Reason)
	}
}

func TestFillBudget(t *testing.T) {
	mk := func(est ...int) []planItem {
		items := make([]planItem, len(est))
		for i, e := range est {
			items[i] = planItem{Number: i + 1, EstimateMinutes: e}
		}
		return items
	}

	t.Run("all fit", func(t *testing.T) {
		plan, deferred, total, over := fillBudget(mk(20, 30, 40), 120)
		if len(plan) != 3 || len(deferred) != 0 {
			t.Fatalf("plan=%d deferred=%d, want 3/0", len(plan), len(deferred))
		}
		if total != 90 || over {
			t.Errorf("total=%d over=%v, want 90/false", total, over)
		}
	})

	t.Run("overflow defers rest in priority order", func(t *testing.T) {
		// budget 60: take 40 (=40), next 30 overflows → 30 and the small 10
		// after it are BOTH deferred (no scan-ahead for smaller fits).
		plan, deferred, total, over := fillBudget(mk(40, 30, 10), 60)
		if len(plan) != 1 || len(deferred) != 2 {
			t.Fatalf("plan=%d deferred=%d, want 1/2", len(plan), len(deferred))
		}
		if total != 40 || over {
			t.Errorf("total=%d over=%v, want 40/false", total, over)
		}
		if deferred[0].EstimateMinutes != 30 || deferred[1].EstimateMinutes != 10 {
			t.Errorf("deferred order wrong: %+v", deferred)
		}
	})

	t.Run("first item alone exceeds budget", func(t *testing.T) {
		plan, deferred, total, over := fillBudget(mk(120, 10), 60)
		if len(plan) != 1 || len(deferred) != 1 {
			t.Fatalf("plan=%d deferred=%d, want 1/1", len(plan), len(deferred))
		}
		if total != 120 || !over {
			t.Errorf("total=%d over=%v, want 120/true", total, over)
		}
	})

	t.Run("empty items", func(t *testing.T) {
		plan, deferred, total, over := fillBudget(nil, 360)
		if plan == nil || deferred == nil {
			t.Fatal("plan and deferred must be non-nil slices")
		}
		if len(plan) != 0 || len(deferred) != 0 || total != 0 || over {
			t.Errorf("got plan=%d deferred=%d total=%d over=%v", len(plan), len(deferred), total, over)
		}
	})
}

func TestFmtMinutes(t *testing.T) {
	tests := []struct {
		m    int
		want string
	}{
		{0, "0m"},
		{45, "45m"},
		{60, "1h"},
		{90, "1h30m"},
		{360, "6h"},
	}
	for _, tt := range tests {
		if got := fmtMinutes(tt.m); got != tt.want {
			t.Errorf("fmtMinutes(%d) = %q, want %q", tt.m, got, tt.want)
		}
	}
}

func TestNewTodayCmd_Wiring(t *testing.T) {
	cmd := newTodayCmd()
	if cmd.Use != "today" {
		t.Errorf("Use = %q, want today", cmd.Use)
	}
	if cmd.Short == "" || cmd.Long == "" {
		t.Error("Short and Long must be set")
	}
	if cmd.Flags().Lookup("budget") == nil {
		t.Error("--budget flag must be defined")
	}
}

func TestNewTodayCmd_InvalidBudget(t *testing.T) {
	for _, bad := range []string{"abc", "0s", "-1h"} {
		cmd := newTodayCmd()
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		cmd.SetArgs([]string{"--budget", bad})
		if err := cmd.Execute(); err == nil {
			t.Errorf("--budget %q: expected error, got nil", bad)
		}
	}
}
