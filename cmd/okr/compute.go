package okr

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

// sourceAliases maps an OKR "計測ソース" key (github:<name>) to the
// corresponding field in the metrics object. Kept in exact sync with okr-hub's
// plugins/okr-progress/scripts/okr_github_metrics.py SOURCE_ALIASES so that
// `gh wheel okr metrics` is a drop-in replacement for that script.
var sourceAliases = map[string]string{
	"github:authored_prs_total":         "authored_prs_total",
	"github:merged_prs":                 "merged_prs",
	"github:avg_cycle_time_hours":       "avg_cycle_time_hours",
	"github:review_comments_received":   "review_comments_received",
	"github:avg_review_comments_per_pr": "avg_review_comments_per_pr",
	"github:reviewed_prs":               "reviewed_prs",
	"github:issues_created":             "issues_created",
	"github:issues_closed":              "issues_closed",
	"github:pr_count":                   "pr_count",
}

// metrics holds the GitHub activity figures for a period. The JSON keys match
// okr-hub's SOURCE_ALIASES values exactly. AvgCycleTimeHours is a pointer so it
// serialises to null (not 0) when no PRs were merged in the period.
type metrics struct {
	AuthoredPRsTotal       int      `json:"authored_prs_total"`
	PRCount                int      `json:"pr_count"`
	MergedPRs              int      `json:"merged_prs"`
	AvgCycleTimeHours      *float64 `json:"avg_cycle_time_hours"`
	ReviewCommentsReceived int      `json:"review_comments_received"`
	AvgReviewCommentsPerPR float64  `json:"avg_review_comments_per_pr"`
	ReviewedPRs            int      `json:"reviewed_prs"`
	IssuesCreated          int      `json:"issues_created"`
	IssuesClosed           int      `json:"issues_closed"`
}

// value returns the metric identified by its metric key (a SOURCE_ALIASES
// value), or nil if the key is unknown. Used to attach a value to a KR.
func (m metrics) value(metricKey string) any {
	switch metricKey {
	case "authored_prs_total":
		return m.AuthoredPRsTotal
	case "pr_count":
		return m.PRCount
	case "merged_prs":
		return m.MergedPRs
	case "avg_cycle_time_hours":
		if m.AvgCycleTimeHours == nil {
			return nil
		}
		return *m.AvgCycleTimeHours
	case "review_comments_received":
		return m.ReviewCommentsReceived
	case "avg_review_comments_per_pr":
		return m.AvgReviewCommentsPerPR
	case "reviewed_prs":
		return m.ReviewedPRs
	case "issues_created":
		return m.IssuesCreated
	case "issues_closed":
		return m.IssuesClosed
	default:
		return nil
	}
}

// krInput is one key result as produced by okr-hub's okr_parse.py.
type krInput struct {
	Label         string `json:"label"`
	Title         string `json:"title"`
	MetricsSource string `json:"metrics_source"`
}

// krMetric is the matched value for a single KR in the output.
type krMetric struct {
	Source  string `json:"source"`
	Value   any    `json:"value"`
	KRTitle string `json:"kr_title"`
}

// parseKRs decodes the --krs JSON array. An empty string yields no KRs.
func parseKRs(s string) ([]krInput, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	var krs []krInput
	if err := json.Unmarshal([]byte(s), &krs); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return krs, nil
}

// matchKRs maps each KR with a recognised metrics_source onto its measured
// value. KRs whose source is not in sourceAliases are skipped. The returned map
// is always non-nil so it marshals to {} rather than null.
func matchKRs(m metrics, krs []krInput) map[string]krMetric {
	out := make(map[string]krMetric, len(krs))
	for _, kr := range krs {
		metricKey, ok := sourceAliases[kr.MetricsSource]
		if !ok {
			continue
		}
		out[kr.Label] = krMetric{
			Source:  kr.MetricsSource,
			Value:   m.value(metricKey),
			KRTitle: kr.Title,
		}
	}
	return out
}

// prItem is the subset of a GitHub Search API item that okr metrics needs.
//
// Comments is the Search API's conversation (issue) comment count for the PR;
// it does NOT include inline code-review comments or review summaries. The
// review_comments_received / avg_review_comments_per_pr metrics are therefore
// conversation-comment figures — this matches okr-hub's okr_github_metrics.py
// (which reads the same field), keeping the values drop-in compatible.
type prItem struct {
	CreatedAt   time.Time `json:"created_at"`
	Comments    int       `json:"comments"`
	PullRequest *struct {
		MergedAt *time.Time `json:"merged_at"`
	} `json:"pull_request"`
}

// reviewCommentStats returns the total comment count across the items and the
// average per item (rounded to 2 dp). An empty slice yields (0, 0).
func reviewCommentStats(items []prItem) (received int, avgPerPR float64) {
	for _, it := range items {
		received += it.Comments
	}
	if len(items) == 0 {
		return 0, 0
	}
	return received, round(float64(received)/float64(len(items)), 2)
}

// cycleTimeAvgHours returns the average created→merged duration in hours across
// merged items, rounded to 1 dp. Returns nil when no item was merged.
func cycleTimeAvgHours(items []prItem) *float64 {
	var total float64
	var n int
	for _, it := range items {
		if it.PullRequest == nil || it.PullRequest.MergedAt == nil || it.CreatedAt.IsZero() {
			continue
		}
		h := it.PullRequest.MergedAt.Sub(it.CreatedAt).Hours()
		if h < 0 {
			continue
		}
		total += h
		n++
	}
	if n == 0 {
		return nil
	}
	avg := round(total/float64(n), 1)
	return &avg
}

// round rounds f to the given number of decimal places.
func round(f float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(f*p) / p
}
