package task

import (
	"strings"
	"time"
)

// knownAIBots lists GitHub App logins that are AI review tools.
var knownAIBots = map[string]bool{
	"copilot[bot]":                  true,
	"github-advanced-security[bot]": true,
	"claude[bot]":                   true,
	"coderabbitai[bot]":             true,
}

// isAIReviewer returns true when the reviewer is a bot or known AI tool.
func isAIReviewer(login, userType string) bool {
	if userType == "Bot" {
		return true
	}
	if strings.HasSuffix(login, "[bot]") {
		return true
	}
	return knownAIBots[login]
}

type reviewStates struct {
	human string
	ai    string
}

// computeReviewStates splits reviews into Human/AI categories and returns
// state icons for each. The 🔄 (addressed) state is triggered when
// latestCommitAt is after the most recent review.
func computeReviewStates(reviews []Review, latestCommitAt time.Time) reviewStates {
	var humanReviews, aiReviews []Review
	for _, r := range reviews {
		if isAIReviewer(r.Author, r.UserType) {
			aiReviews = append(aiReviews, r)
		} else {
			humanReviews = append(humanReviews, r)
		}
	}

	var latestReviewAt time.Time
	for _, r := range reviews {
		if r.SubmittedAt.After(latestReviewAt) {
			latestReviewAt = r.SubmittedAt
		}
	}

	addressedAfter := !latestCommitAt.IsZero() && !latestReviewAt.IsZero() &&
		latestCommitAt.After(latestReviewAt)

	return reviewStates{
		human: stateForReviews(humanReviews, addressedAfter),
		ai:    stateForReviews(aiReviews, addressedAfter) + "🤖",
	}
}

// stateForReviews computes the review state icon from a reviewer set.
// Priority: 🔄 > 🚫 > ✅ > 💬 > ⏳
func stateForReviews(reviews []Review, addressedAfter bool) string {
	if len(reviews) == 0 {
		return "⏳"
	}
	if addressedAfter {
		return "🔄"
	}

	// Use the latest review per author to avoid counting stale reviews.
	latest := make(map[string]Review, len(reviews))
	for _, r := range reviews {
		if ex, ok := latest[r.Author]; !ok || r.SubmittedAt.After(ex.SubmittedAt) {
			latest[r.Author] = r
		}
	}

	hasChangesRequested := false
	allApproved := true
	for _, r := range latest {
		switch r.State {
		case "CHANGES_REQUESTED":
			hasChangesRequested = true
			allApproved = false
		case "APPROVED":
			// counts as approved, allApproved stays true
		default: // COMMENTED, PENDING, DISMISSED
			allApproved = false
		}
	}

	switch {
	case hasChangesRequested:
		return "🚫"
	case allApproved:
		return "✅"
	default:
		return "💬"
	}
}

// humanReviewClass is the actionable state of a PR derived from human reviews
// only. It is computed independently from the display icons in
// computeReviewStates so that `task today` can classify own PRs without the
// 🔄 short-circuit conflating human/AI reviews.
type humanReviewClass int

const (
	reviewNone             humanReviewClass = iota // no human reviews yet
	reviewApproved                                 // ≥1 human approval, no outstanding changes requested
	reviewChangesRequested                         // a human requested changes, not yet addressed by a newer commit
	reviewAddressed                                // changes requested, but a commit landed after → awaiting re-review
	reviewCommented                                // only comments from humans (no approval / changes requested)
)

// classifyHumanReview reduces a PR's human reviews to a single actionable state.
//
// Decision reviews (APPROVED / CHANGES_REQUESTED / DISMISSED) and plain
// COMMENTED reviews are tracked separately per author: on GitHub a later
// COMMENTED review does NOT clear an earlier approval or change request — the
// decision sticks. A human approval is likewise sticky across new commits
// (kept until dismissed or re-requested). Changes-requested becomes
// reviewAddressed once latestCommitAt is newer than the most recent
// changes-requested review, so a PR you have already pushed a fix for is not
// surfaced as still-needing-work. AI/bot reviews are ignored.
func classifyHumanReview(reviews []Review, latestCommitAt time.Time) humanReviewClass {
	latestDecision := make(map[string]Review, len(reviews))
	latestComment := make(map[string]Review, len(reviews))
	for _, r := range reviews {
		if isAIReviewer(r.Author, r.UserType) {
			continue
		}
		switch r.State {
		case "APPROVED", "CHANGES_REQUESTED", "DISMISSED":
			if ex, ok := latestDecision[r.Author]; !ok || r.SubmittedAt.After(ex.SubmittedAt) {
				latestDecision[r.Author] = r
			}
		case "COMMENTED":
			if ex, ok := latestComment[r.Author]; !ok || r.SubmittedAt.After(ex.SubmittedAt) {
				latestComment[r.Author] = r
			}
		}
	}
	if len(latestDecision) == 0 && len(latestComment) == 0 {
		return reviewNone
	}

	var (
		hasChangesRequested bool
		hasApproved         bool
		latestChangesAt     time.Time
	)
	for _, r := range latestDecision {
		switch r.State {
		case "CHANGES_REQUESTED":
			hasChangesRequested = true
			if r.SubmittedAt.After(latestChangesAt) {
				latestChangesAt = r.SubmittedAt
			}
		case "APPROVED":
			hasApproved = true
		}
		// DISMISSED carries no standing decision.
	}

	switch {
	case hasChangesRequested:
		if !latestCommitAt.IsZero() && latestCommitAt.After(latestChangesAt) {
			return reviewAddressed
		}
		return reviewChangesRequested
	case hasApproved:
		return reviewApproved
	case len(latestComment) > 0:
		return reviewCommented
	default:
		return reviewNone
	}
}
