package okr

import (
	"fmt"
	"net/url"

	"github.com/UtakataKyosui/gh-wheel/internal/ghclient"
)

// maxPages bounds enumeration for the average computations. The GitHub Search
// API caps pagination at 1000 results (page * per_page ≤ 1000) regardless, so
// 10 pages of 100 is the practical ceiling. Count metrics use total_count and
// are exact beyond this limit; only the averages are computed over the sample.
const maxPages = 10

type countPage struct {
	TotalCount int `json:"total_count"`
}

type itemsPage struct {
	TotalCount int      `json:"total_count"`
	Items      []prItem `json:"items"`
}

// searchEndpoint builds a search/issues endpoint for the given query.
// url.Values.Encode encodes spaces as '+', the correct form for the q param.
func searchEndpoint(q string, perPage, page int) string {
	v := url.Values{
		"q":        {q},
		"per_page": {fmt.Sprintf("%d", perPage)},
		"page":     {fmt.Sprintf("%d", page)},
	}
	return "search/issues?" + v.Encode()
}

// countItems returns the exact total_count for a query with a single-item page.
func countItems(c *ghclient.Client, q string) (int, error) {
	var page countPage
	if err := c.Get(searchEndpoint(q, 1, 1), &page); err != nil {
		return 0, err
	}
	return page.TotalCount, nil
}

// enumerateItems pages through a query (up to maxPages) and returns the items
// together with the query's exact total match count. The first page's response
// already carries total_count (reported independently of pagination), so this
// doubles as a count for the authored/merged queries — no extra countItems call
// is needed for those. The items slice is capped at maxPages*100; total stays
// exact even beyond that cap.
func enumerateItems(c *ghclient.Client, q string) (items []prItem, total int, err error) {
	for page := 1; page <= maxPages; page++ {
		var p itemsPage
		if err := c.Get(searchEndpoint(q, 100, page), &p); err != nil {
			return nil, 0, err
		}
		if page == 1 {
			total = p.TotalCount
		}
		items = append(items, p.Items...)
		if len(p.Items) < 100 || len(items) >= p.TotalCount {
			break
		}
	}
	return items, total, nil
}

// gatherMetrics computes the full metrics set for the period. When repo is
// non-empty (owner/name), every query is scoped to that repository; otherwise
// the search is cross-repo (every repository the user can see).
//
// Requests are issued sequentially: the GitHub Search API enforces a strict
// secondary rate limit, so parallel search calls risk 403s.
func gatherMetrics(c *ghclient.Client, repo, since, until string) (metrics, error) {
	scoped := func(base string) string {
		if repo != "" {
			return "repo:" + repo + " " + base
		}
		return base
	}
	dateRange := func(field string) string {
		return fmt.Sprintf("%s:%s..%s", field, since, until)
	}

	// Enumerating authored/merged PRs already yields each query's total_count on
	// its first page, so pr_count and merged_prs come for free — no separate
	// countItems request for these two (fewer Search calls = less rate-limit risk).
	authored, prCount, err := enumerateItems(c, scoped("is:pr author:@me "+dateRange("created")))
	if err != nil {
		return metrics{}, err
	}
	merged, mergedCount, err := enumerateItems(c, scoped("is:pr author:@me "+dateRange("merged")))
	if err != nil {
		return metrics{}, err
	}

	reviewedCount, err := countItems(c, scoped("is:pr reviewed-by:@me "+dateRange("created")))
	if err != nil {
		return metrics{}, err
	}
	issuesCreated, err := countItems(c, scoped("is:issue author:@me "+dateRange("created")))
	if err != nil {
		return metrics{}, err
	}
	issuesClosed, err := countItems(c, scoped("is:issue author:@me "+dateRange("closed")))
	if err != nil {
		return metrics{}, err
	}

	// review_comments_received / avg_review_comments_per_pr are computed over the
	// enumerated sample (capped at maxPages*100). If authored PRs exceed that cap
	// the average is a sample mean, while pr_count stays exact via total_count.
	received, avgPerPR := reviewCommentStats(authored)

	return metrics{
		AuthoredPRsTotal:       prCount,
		PRCount:                prCount,
		MergedPRs:              mergedCount,
		AvgCycleTimeHours:      cycleTimeAvgHours(merged),
		ReviewCommentsReceived: received,
		AvgReviewCommentsPerPR: avgPerPR,
		ReviewedPRs:            reviewedCount,
		IssuesCreated:          issuesCreated,
		IssuesClosed:           issuesClosed,
	}, nil
}
