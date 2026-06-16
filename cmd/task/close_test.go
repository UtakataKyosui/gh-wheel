package task

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/UtakataKyosui/gh-wheel/internal/ghclient"
)

// newTestClient creates a *ghclient.Client that routes all HTTP traffic to srv.
func newTestClient(t *testing.T, srv *httptest.Server, owner, name string) *ghclient.Client {
	t.Helper()
	transport := &ghclient.TestTransport{
		Inner:   srv.Client().Transport,
		BaseURL: srv.URL,
	}
	c, err := ghclient.NewForTest(transport, owner, name)
	if err != nil {
		t.Fatalf("newTestClient: %v", err)
	}
	return c
}

// mockIssueServer creates an httptest.Server that responds to GitHub Issues API calls.
// state is the state to return for GET; expectPatch controls whether a PATCH is expected.
func mockIssueServer(t *testing.T, issueNum int, state string, expectPatch bool) *httptest.Server {
	t.Helper()
	patched := false
	mux := http.NewServeMux()

	getPath := fmt.Sprintf("/repos/owner/repo/issues/%d", issueNum)
	mux.HandleFunc(getPath, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			resp := issueState{
				State:   state,
				Title:   fmt.Sprintf("Issue #%d title", issueNum),
				HTMLURL: fmt.Sprintf("https://github.com/owner/repo/issues/%d", issueNum),
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Errorf("encode response: %v", err)
			}
		case http.MethodPatch:
			if !expectPatch {
				t.Errorf("unexpected PATCH for already-closed issue")
			}
			patched = true
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode PATCH body: %v", err)
			}
			if body["state"] != "closed" {
				t.Errorf("PATCH body state: want closed, got %q", body["state"])
			}
			resp := issueState{
				State:   "closed",
				Title:   fmt.Sprintf("Issue #%d title", issueNum),
				HTMLURL: fmt.Sprintf("https://github.com/owner/repo/issues/%d", issueNum),
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Errorf("encode patch response: %v", err)
			}
		default:
			http.NotFound(w, r)
		}
	})

	srv := httptest.NewTLSServer(mux)
	t.Cleanup(func() {
		srv.Close()
		if expectPatch && !patched {
			t.Errorf("expected PATCH request but none received")
		}
	})
	return srv
}

// TestConfirmAndClose_Match tests that when the user inputs the correct number,
// the issue is closed successfully.
func TestConfirmAndClose_Match(t *testing.T) {
	issueNum := 42
	srv := mockIssueServer(t, issueNum, "open", true)

	c := newTestClient(t, srv, "owner", "repo")
	in := strings.NewReader("42\n")
	var out bytes.Buffer

	err := confirmAndClose(c, issueNum, false, in, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotOut := out.String()
	if !strings.Contains(gotOut, "closed") && !strings.Contains(gotOut, "Closed") {
		t.Errorf("output should mention closed, got: %q", gotOut)
	}
}

// TestConfirmAndClose_Mismatch tests that when the user inputs the wrong number,
// an error is returned.
func TestConfirmAndClose_Mismatch(t *testing.T) {
	issueNum := 99
	srv := mockIssueServer(t, issueNum, "open", false)

	c := newTestClient(t, srv, "owner", "repo")
	in := strings.NewReader("88\n")
	var out bytes.Buffer

	err := confirmAndClose(c, issueNum, false, in, &out)
	if err == nil {
		t.Fatal("expected error for mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "confirmation") && !strings.Contains(err.Error(), "mismatch") && !strings.Contains(err.Error(), "match") {
		t.Errorf("error message should mention confirmation mismatch, got: %q", err.Error())
	}
}

// TestConfirmAndClose_AlreadyClosed tests that when the issue is already closed,
// no PATCH is made and success is returned.
func TestConfirmAndClose_AlreadyClosed(t *testing.T) {
	issueNum := 7
	srv := mockIssueServer(t, issueNum, "closed", false)

	c := newTestClient(t, srv, "owner", "repo")
	in := strings.NewReader("7\n")
	var out bytes.Buffer

	err := confirmAndClose(c, issueNum, false, in, &out)
	if err != nil {
		t.Fatalf("unexpected error for already-closed issue: %v", err)
	}

	gotOut := out.String()
	if !strings.Contains(gotOut, "Already closed") && !strings.Contains(gotOut, "already closed") {
		t.Errorf("output should mention already closed, got: %q", gotOut)
	}
}

// TestConfirmAndClose_JSONMode tests that with jsonMode=true, no confirmation prompt is needed.
func TestConfirmAndClose_JSONMode(t *testing.T) {
	issueNum := 55
	srv := mockIssueServer(t, issueNum, "open", true)

	c := newTestClient(t, srv, "owner", "repo")
	// Provide empty stdin since --json skips confirmation
	in := strings.NewReader("")
	var out bytes.Buffer

	err := confirmAndClose(c, issueNum, true, in, &out)
	if err != nil {
		t.Fatalf("unexpected error in json mode: %v", err)
	}
}
