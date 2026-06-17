package feedback

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/UtakataKyosui/gh-wheel/internal/ghclient"
)

// ─── HTTP helpers ─────────────────────────────────────────────────────────────

func newFeedbackTestClient(t *testing.T, srv *httptest.Server) *ghclient.Client {
	t.Helper()
	transport := &ghclient.TestTransport{
		Inner:   srv.Client().Transport,
		BaseURL: srv.URL,
	}
	c, err := ghclient.NewForTest(transport, "UtakataKyosui", "gh-wheel")
	if err != nil {
		t.Fatalf("NewForTest: %v", err)
	}
	return c
}

func mockIssueCreateServer(t *testing.T, wantLabel string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/UtakataKyosui/gh-wheel/issues", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
			return
		}
		labels, _ := body["labels"].([]any)
		if len(labels) == 0 || labels[0].(string) != wantLabel {
			t.Errorf("label: want %q, got %v", wantLabel, labels)
		}
		resp := map[string]string{"html_url": "https://github.com/UtakataKyosui/gh-wheel/issues/99"}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode: %v", err)
		}
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// ─── submitFeedback ───────────────────────────────────────────────────────────

func TestSubmitFeedback_Feature(t *testing.T) {
	srv := mockIssueCreateServer(t, "enhancement")
	c := newFeedbackTestClient(t, srv)

	url, err := submitFeedback(c, &feedbackResult{kind: kindFeature, title: "add TUI", body: "nice"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(url, "issues/99") {
		t.Errorf("URL: want issues/99, got %q", url)
	}
}

func TestSubmitFeedback_Bug(t *testing.T) {
	srv := mockIssueCreateServer(t, "bug")
	c := newFeedbackTestClient(t, srv)

	_, err := submitFeedback(c, &feedbackResult{kind: kindBug, title: "crash on startup"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ─── Command structure ────────────────────────────────────────────────────────

func TestNewCmd_UsageAndShort(t *testing.T) {
	cmd := NewCmd()
	if cmd.Use != "feedback" {
		t.Errorf("Use: want %q, got %q", "feedback", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if err := cmd.Args(cmd, []string{"unexpected"}); err == nil {
		t.Error("command should reject positional arguments")
	}
}
