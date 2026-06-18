package okr

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/UtakataKyosui/gh-wheel/internal/ghclient"
)

// searchResponse is the minimal Search API envelope the mock returns.
type searchResponse struct {
	TotalCount int           `json:"total_count"`
	Items      []interface{} `json:"items"`
}

func prJSON(createdAt, mergedAt string, comments int) map[string]any {
	item := map[string]any{"created_at": createdAt, "comments": comments}
	if mergedAt != "" {
		item["pull_request"] = map[string]any{"merged_at": mergedAt}
	} else {
		item["pull_request"] = map[string]any{}
	}
	return item
}

// mockSearchServer answers /search/issues, dispatching on the q parameter, and
// records every query it saw.
func mockSearchServer(t *testing.T, queries *[]string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/search/issues", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if queries != nil {
			*queries = append(*queries, q)
		}

		var resp searchResponse
		switch {
		case strings.Contains(q, "reviewed-by:"):
			resp = searchResponse{TotalCount: 3}
		case strings.Contains(q, "is:issue"):
			if strings.Contains(q, "closed:") {
				resp = searchResponse{TotalCount: 5}
			} else {
				resp = searchResponse{TotalCount: 4}
			}
		case strings.Contains(q, "merged:"):
			resp = searchResponse{
				TotalCount: 1,
				Items: []interface{}{
					prJSON("2026-01-01T00:00:00Z", "2026-01-01T02:00:00Z", 0),
				},
			}
		default: // authored, created
			resp = searchResponse{
				TotalCount: 2,
				Items: []interface{}{
					prJSON("2026-01-02T00:00:00Z", "", 3),
					prJSON("2026-01-03T00:00:00Z", "", 1),
				},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode: %v", err)
		}
	})

	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newTestClient(t *testing.T, srv *httptest.Server) *ghclient.Client {
	t.Helper()
	transport := &ghclient.TestTransport{Inner: srv.Client().Transport, BaseURL: srv.URL}
	c, err := ghclient.NewForTest(transport, "", "")
	if err != nil {
		t.Fatalf("newTestClient: %v", err)
	}
	return c
}

func TestGatherMetrics(t *testing.T) {
	srv := mockSearchServer(t, nil)
	c := newTestClient(t, srv)

	m, err := gatherMetrics(c, "", "2026-01-01", "2026-06-30")
	if err != nil {
		t.Fatalf("gatherMetrics: %v", err)
	}

	checks := []struct {
		name string
		got  int
		want int
	}{
		{"authored_prs_total", m.AuthoredPRsTotal, 2},
		{"pr_count", m.PRCount, 2},
		{"merged_prs", m.MergedPRs, 1},
		{"reviewed_prs", m.ReviewedPRs, 3},
		{"issues_created", m.IssuesCreated, 4},
		{"issues_closed", m.IssuesClosed, 5},
		{"review_comments_received", m.ReviewCommentsReceived, 4},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}

	if m.AvgReviewCommentsPerPR != 2.0 {
		t.Errorf("avg_review_comments_per_pr = %v, want 2.0", m.AvgReviewCommentsPerPR)
	}
	if m.AvgCycleTimeHours == nil || *m.AvgCycleTimeHours != 2.0 {
		t.Errorf("avg_cycle_time_hours = %v, want 2.0", m.AvgCycleTimeHours)
	}
}

func TestGatherMetrics_RepoScoped(t *testing.T) {
	var queries []string
	srv := mockSearchServer(t, &queries)
	c := newTestClient(t, srv)

	if _, err := gatherMetrics(c, "owner/repo", "2026-01-01", "2026-06-30"); err != nil {
		t.Fatalf("gatherMetrics: %v", err)
	}

	if len(queries) == 0 {
		t.Fatal("no queries recorded")
	}
	for _, q := range queries {
		if !strings.Contains(q, "repo:owner/repo") {
			t.Errorf("query missing repo qualifier: %q", q)
		}
	}
}

func TestEnumerateItems_Pagination(t *testing.T) {
	// total_count = 150: page 1 returns 100 items, page 2 returns 50, then stops.
	mux := http.NewServeMux()
	mux.HandleFunc("/search/issues", func(w http.ResponseWriter, r *http.Request) {
		n := 0
		switch r.URL.Query().Get("page") {
		case "1":
			n = 100
		case "2":
			n = 50
		}
		items := make([]interface{}, 0, n)
		for i := 0; i < n; i++ {
			items = append(items, prJSON("2026-01-02T00:00:00Z", "", 1))
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(searchResponse{TotalCount: 150, Items: items}); err != nil {
			t.Errorf("encode: %v", err)
		}
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	c := newTestClient(t, srv)

	items, total, err := enumerateItems(c, "is:pr author:@me")
	if err != nil {
		t.Fatalf("enumerateItems: %v", err)
	}
	if total != 150 {
		t.Errorf("total = %d, want 150 (from page-1 total_count)", total)
	}
	if len(items) != 150 {
		t.Errorf("items = %d, want 150 (page1=100 + page2=50)", len(items))
	}
}

func TestGatherMetrics_CrossRepoNoQualifier(t *testing.T) {
	var queries []string
	srv := mockSearchServer(t, &queries)
	c := newTestClient(t, srv)

	if _, err := gatherMetrics(c, "", "2026-01-01", "2026-06-30"); err != nil {
		t.Fatalf("gatherMetrics: %v", err)
	}
	for _, q := range queries {
		if strings.Contains(q, "repo:") {
			t.Errorf("cross-repo query should not contain repo qualifier: %q", q)
		}
	}
}
