package task

import (
	"testing"
	"time"
)

func TestIsAIReviewer(t *testing.T) {
	cases := []struct {
		login    string
		userType string
		want     bool
	}{
		{"alice", "User", false},
		{"copilot[bot]", "Bot", true},
		{"github-actions[bot]", "Bot", true},
		{"any-name[bot]", "User", true},  // [bot] suffix wins
		{"coderabbitai[bot]", "User", true}, // known list
		{"claude[bot]", "Bot", true},
		{"bob", "Bot", true}, // type=Bot wins even without [bot] suffix
	}
	for _, tc := range cases {
		got := isAIReviewer(tc.login, tc.userType)
		if got != tc.want {
			t.Errorf("isAIReviewer(%q, %q) = %v, want %v", tc.login, tc.userType, got, tc.want)
		}
	}
}

func TestStateForReviews_Empty(t *testing.T) {
	got := stateForReviews(nil, false)
	if got != "⏳" {
		t.Errorf("empty reviews: want ⏳, got %q", got)
	}
}

func TestStateForReviews_AddressedAfter(t *testing.T) {
	reviews := []Review{{Author: "alice", State: "CHANGES_REQUESTED", SubmittedAt: time.Now()}}
	got := stateForReviews(reviews, true)
	if got != "🔄" {
		t.Errorf("addressedAfter=true: want 🔄, got %q", got)
	}
}

func TestStateForReviews_ChangesRequested(t *testing.T) {
	reviews := []Review{{Author: "alice", State: "CHANGES_REQUESTED", SubmittedAt: time.Now()}}
	got := stateForReviews(reviews, false)
	if got != "🚫" {
		t.Errorf("CHANGES_REQUESTED: want 🚫, got %q", got)
	}
}

func TestStateForReviews_AllApproved(t *testing.T) {
	now := time.Now()
	reviews := []Review{
		{Author: "alice", State: "APPROVED", SubmittedAt: now},
		{Author: "bob", State: "APPROVED", SubmittedAt: now},
	}
	got := stateForReviews(reviews, false)
	if got != "✅" {
		t.Errorf("all APPROVED: want ✅, got %q", got)
	}
}

func TestStateForReviews_CommentOnly(t *testing.T) {
	reviews := []Review{{Author: "alice", State: "COMMENTED", SubmittedAt: time.Now()}}
	got := stateForReviews(reviews, false)
	if got != "💬" {
		t.Errorf("COMMENTED: want 💬, got %q", got)
	}
}

func TestStateForReviews_MixedPriority(t *testing.T) {
	now := time.Now()
	reviews := []Review{
		{Author: "alice", State: "APPROVED", SubmittedAt: now},
		{Author: "bob", State: "CHANGES_REQUESTED", SubmittedAt: now},
	}
	got := stateForReviews(reviews, false)
	if got != "🚫" {
		t.Errorf("APPROVED+CHANGES_REQUESTED: want 🚫 (priority), got %q", got)
	}
}

func TestStateForReviews_LatestReviewPerAuthor(t *testing.T) {
	older := time.Now().Add(-1 * time.Hour)
	newer := time.Now()
	// alice changed to APPROVED after earlier CHANGES_REQUESTED
	reviews := []Review{
		{Author: "alice", State: "CHANGES_REQUESTED", SubmittedAt: older},
		{Author: "alice", State: "APPROVED", SubmittedAt: newer},
	}
	got := stateForReviews(reviews, false)
	if got != "✅" {
		t.Errorf("latest review should win: want ✅, got %q", got)
	}
}

func TestComputeReviewStates_HumanAndAI(t *testing.T) {
	now := time.Now()
	reviews := []Review{
		{Author: "alice", UserType: "User", State: "APPROVED", SubmittedAt: now},
		{Author: "copilot[bot]", UserType: "Bot", State: "CHANGES_REQUESTED", SubmittedAt: now},
	}
	rs := computeReviewStates(reviews, time.Time{})
	if rs.human != "✅" {
		t.Errorf("human state: want ✅, got %q", rs.human)
	}
	if rs.ai != "🚫🤖" {
		t.Errorf("ai state: want 🚫🤖, got %q", rs.ai)
	}
}

func TestComputeReviewStates_AddressedAfter(t *testing.T) {
	reviewTime := time.Now().Add(-30 * time.Minute)
	commitTime := time.Now()
	reviews := []Review{
		{Author: "alice", UserType: "User", State: "CHANGES_REQUESTED", SubmittedAt: reviewTime},
	}
	rs := computeReviewStates(reviews, commitTime)
	if rs.human != "🔄" {
		t.Errorf("human after commit push: want 🔄, got %q", rs.human)
	}
}

func TestComputeReviewStates_NoAIReviewers(t *testing.T) {
	reviews := []Review{
		{Author: "alice", UserType: "User", State: "APPROVED", SubmittedAt: time.Now()},
	}
	rs := computeReviewStates(reviews, time.Time{})
	if rs.ai != "⏳🤖" {
		t.Errorf("no AI reviews: want ⏳🤖, got %q", rs.ai)
	}
}
