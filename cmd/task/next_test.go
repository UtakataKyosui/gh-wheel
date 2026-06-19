package task

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/UtakataKyosui/gh-wheel/internal/cliexit"
)

func TestBuildCandidateQuery(t *testing.T) {
	t.Run("default excludes non-actionable labels and linked PRs", func(t *testing.T) {
		q := buildCandidateQuery("owner/repo", "", false)
		for _, want := range []string{
			"is:issue", "is:open", "no:assignee", "repo:owner/repo",
			"-label:epic", "-label:wontfix", "-label:invalid", "-label:duplicate", "-label:question",
			"-linked:pr",
		} {
			if !strings.Contains(q, want) {
				t.Errorf("query missing %q\nfull: %s", want, q)
			}
		}
	})

	t.Run("no-blockers drops -linked:pr", func(t *testing.T) {
		q := buildCandidateQuery("owner/repo", "", true)
		if strings.Contains(q, "-linked:pr") {
			t.Errorf("--no-blockers should drop -linked:pr: %s", q)
		}
	})

	t.Run("label filter is added", func(t *testing.T) {
		q := buildCandidateQuery("owner/repo", "good first issue", false)
		if !strings.Contains(q, `label:"good first issue"`) {
			t.Errorf("query missing quoted label filter: %s", q)
		}
	})
}

func TestScoreLabels(t *testing.T) {
	tests := []struct {
		labels []string
		want   int
	}{
		{nil, 0},
		{[]string{"enhancement"}, 0},
		{[]string{"good first issue"}, 2},
		{[]string{"help wanted"}, 1},
		{[]string{"priority:low"}, -2},
		{[]string{"good first issue", "help wanted"}, 3},
		{[]string{"good first issue", "priority:low"}, 0},
	}
	for _, tt := range tests {
		if got := scoreLabels(tt.labels); got != tt.want {
			t.Errorf("scoreLabels(%v) = %d, want %d", tt.labels, got, tt.want)
		}
	}
}

func TestIsExcludedLabel(t *testing.T) {
	if !isExcludedLabel([]string{"enhancement", "epic"}) {
		t.Error("epic should be excluded")
	}
	if !isExcludedLabel([]string{"WontFix"}) {
		t.Error("label match should be case-insensitive")
	}
	if isExcludedLabel([]string{"enhancement", "area:task"}) {
		t.Error("actionable labels should not be excluded")
	}
}

func TestRankCandidates(t *testing.T) {
	in := []candidate{
		{Number: 8, Score: -2},
		{Number: 20, Score: 0},
		{Number: 16, Score: 0},
		{Number: 5, Score: 2},
	}
	got := rankCandidates(in)
	wantOrder := []int{5, 16, 20, 8} // score desc, then number asc
	for i, w := range wantOrder {
		if got[i].Number != w {
			t.Errorf("rank[%d] = #%d, want #%d (full: %+v)", i, got[i].Number, w, got)
		}
	}
}

func TestGatherCandidates(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/search/issues", func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"total_count": 3,
			"items": []map[string]any{
				{"number": 20, "title": "GHES", "html_url": "u20", "state": "open",
					"labels": []map[string]string{{"name": "enhancement"}}},
				{"number": 15, "title": "graph TUI", "html_url": "u15", "state": "open",
					"labels": []map[string]string{{"name": "enhancement"}, {"name": "priority:low"}}},
				{"number": 99, "title": "aggregator", "html_url": "u99", "state": "open",
					"labels": []map[string]string{{"name": "good first issue"}}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	c := newTestClient(t, srv, "owner", "repo")

	// Stub: #99 has an open sub-issue (aggregator) -> excluded.
	check := func(num int) (bool, error) { return num == 99, nil }

	cands, err := gatherCandidates(c, "owner/repo", "", false, check, 0 /* no limit */)
	if err != nil {
		t.Fatalf("gatherCandidates: %v", err)
	}
	if len(cands) != 2 {
		t.Fatalf("expected 2 candidates (#99 excluded as aggregator), got %d: %+v", len(cands), cands)
	}
	// #20 (score 0) ranks above #15 (score -2 from priority:low).
	if cands[0].Number != 20 || cands[1].Number != 15 {
		t.Errorf("unexpected ranking: %+v", cands)
	}
}

func TestGatherCandidates_LimitStopsBlockerChecks(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/search/issues", func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"total_count": 3,
			"items": []map[string]any{
				{"number": 5, "title": "a", "html_url": "u5", "state": "open",
					"labels": []map[string]string{{"name": "good first issue"}}}, // score 2 -> ranks first
				{"number": 16, "title": "b", "html_url": "u16", "state": "open"},
				{"number": 20, "title": "c", "html_url": "u20", "state": "open"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	c := newTestClient(t, srv, "owner", "repo")

	// Record which issues had their blocker checked; none are blocked.
	var checked []int
	check := func(num int) (bool, error) {
		checked = append(checked, num)
		return false, nil
	}

	cands, err := gatherCandidates(c, "owner/repo", "", false, check, 1)
	if err != nil {
		t.Fatalf("gatherCandidates: %v", err)
	}
	if len(cands) != 1 || cands[0].Number != 5 {
		t.Fatalf("expected only top candidate #5, got %+v", cands)
	}
	// Lazy filtering must stop after the first non-blocked candidate; the other
	// two must never hit the (expensive) blocker check.
	if len(checked) != 1 || checked[0] != 5 {
		t.Errorf("expected blocker check on #5 only, got %v", checked)
	}
}

func TestStatusWriter(t *testing.T) {
	// In JSON mode, status/success messages must not go to stdout (which is
	// reserved for the JSON document); they go to stderr instead.
	if statusWriter(true) != os.Stderr {
		t.Error("statusWriter(true) should be os.Stderr so stdout stays pure JSON")
	}
	if statusWriter(false) != os.Stdout {
		t.Error("statusWriter(false) should be os.Stdout")
	}
}

func TestConfirmAndAssign_Mismatch(t *testing.T) {
	cand := candidate{Number: 42, Title: "x", URL: "u"}
	// Wrong confirmation input; client is never used on the mismatch path.
	_, err := confirmAndAssign(nil, cand, "octocat", false, strings.NewReader("99\n"), &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected confirmation mismatch error")
	}
}

func TestConfirmAndAssign_Success(t *testing.T) {
	issueNum := 55
	posted := false
	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/repos/owner/repo/issues/%d/assignees", issueNum),
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.NotFound(w, r)
				return
			}
			posted = true
			var body map[string][]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if got := body["assignees"]; len(got) != 1 || got[0] != "octocat" {
				t.Errorf("assignees payload = %v, want [octocat]", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	c := newTestClient(t, srv, "owner", "repo")

	cand := candidate{Number: issueNum, Title: "do thing", URL: "u55"}
	var out bytes.Buffer
	asg, err := confirmAndAssign(c, cand, "octocat", true /* skip confirm */, strings.NewReader(""), &out)
	if err != nil {
		t.Fatalf("confirmAndAssign: %v", err)
	}
	if !posted {
		t.Error("expected POST to assignees endpoint")
	}
	if asg == nil || asg.Number != issueNum || asg.Assignee != "octocat" {
		t.Errorf("unexpected assigned: %+v", asg)
	}
}

func TestAssignExplicit_RejectsPR(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/issues/7", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"title":"a pr","html_url":"u7","state":"open","pull_request":{"url":"x"}}`))
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	c := newTestClient(t, srv, "owner", "repo")

	err := assignExplicit(c, "owner/repo", 7, true /*json*/, "", false, true)
	if err == nil || !strings.Contains(err.Error(), "pull request") {
		t.Fatalf("expected pull-request rejection, got %v", err)
	}
}

func TestAssignExplicit_RejectsClosed(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/issues/8", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"title":"closed","html_url":"u8","state":"closed"}`))
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	c := newTestClient(t, srv, "owner", "repo")

	err := assignExplicit(c, "owner/repo", 8, true, "", false, true)
	if err == nil || !strings.Contains(err.Error(), "not open") {
		t.Fatalf("expected not-open error, got %v", err)
	}
	// The Issue exists, so the machine-readable code must say ISSUE_NOT_OPEN,
	// not NOT_FOUND.
	var ce *cliexit.Error
	if !errors.As(err, &ce) || ce.Code != cliexit.ErrCodeIssueNotOpen {
		t.Errorf("expected code ISSUE_NOT_OPEN, got %+v", err)
	}
}

func TestAssignExplicit_DryRunDoesNotAssign(t *testing.T) {
	posted := false
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/issues/9", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"title":"ok","html_url":"u9","state":"open"}`))
	})
	mux.HandleFunc("/repos/owner/repo/issues/9/assignees", func(w http.ResponseWriter, _ *http.Request) {
		posted = true
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	c := newTestClient(t, srv, "owner", "repo")

	if err := assignExplicit(c, "owner/repo", 9, true /*json*/, "", true /*dryRun*/, false); err != nil {
		t.Fatalf("dry-run assignExplicit: %v", err)
	}
	if posted {
		t.Error("dry-run must not POST to the assignees endpoint")
	}
}

func TestAssignExplicit_Success(t *testing.T) {
	posted := false
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/issues/10", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"title":"ok","html_url":"u10","state":"open"}`))
	})
	mux.HandleFunc("/repos/owner/repo/issues/10/assignees", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posted = true
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})
	mux.HandleFunc("/user", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"login":"octocat"}`))
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	c := newTestClient(t, srv, "owner", "repo")

	if err := assignExplicit(c, "owner/repo", 10, true /*json*/, "", false, true /*yes*/); err != nil {
		t.Fatalf("assignExplicit: %v", err)
	}
	if !posted {
		t.Error("expected POST to the assignees endpoint")
	}
}
