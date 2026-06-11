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
