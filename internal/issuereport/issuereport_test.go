package issuereport

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/UtakataKyosui/gh-wheel/internal/cliexit"
	"github.com/UtakataKyosui/gh-wheel/internal/ghclient"
)

func TestRedactArgs(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"flag with space value", []string{"--body", "secret"}, []string{"--body", redactedValue}},
		{"flag with inline value", []string{"--body=secret"}, []string{"--body=" + redactedValue}},
		{"short flag value", []string{"-R", "org/private"}, []string{"-R", redactedValue}},
		{"positional", []string{"123"}, []string{redactedValue}},
		{"stdin marker preserved", []string{"-"}, []string{"-"}},
		{"-- switches to positional-only", []string{"--", "-x", "secret"}, []string{"--", redactedValue, redactedValue}},
		{"secret after -- is redacted", []string{"--", "--token", "ghp_x"}, []string{"--", redactedValue, redactedValue}},
		{"mixed", []string{"--title", "私の秘密", "-j", "42"}, []string{"--title", redactedValue, "-j", redactedValue}},
		{"empty", nil, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RedactArgs(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("len: got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("[%d]: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestReportable(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"general", cliexit.NewGeneral(fmt.Errorf("boom")), true},
		{"auth", cliexit.NewAuth(cliexit.ErrCodeAuthNoToken, fmt.Errorf("x")), false},
		{"usage", cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs, fmt.Errorf("x")), false},
		{"validation", cliexit.NewValidation(cliexit.ErrCodeValidation, fmt.Errorf("x"), nil), false},
		{"api", cliexit.NewAPI(cliexit.ErrCodeAPI, fmt.Errorf("x")), false},
		{"non-structured", fmt.Errorf("raw"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Reportable(tc.err); got != tc.want {
				t.Errorf("Reportable(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestSuppressed(t *testing.T) {
	cases := []struct {
		name        string
		asJSON      bool
		interactive bool
		optOut      bool
		want        bool
	}{
		{"interactive only → not suppressed", false, true, false, false},
		{"json suppresses", true, true, false, true},
		{"non-interactive suppresses", false, false, false, true},
		{"opt-out suppresses", false, true, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Suppressed(tc.asJSON, tc.interactive, tc.optOut); got != tc.want {
				t.Errorf("Suppressed(%v,%v,%v) = %v, want %v",
					tc.asJSON, tc.interactive, tc.optOut, got, tc.want)
			}
		})
	}
}

func TestBuildIssue_Error(t *testing.T) {
	c := Context{
		Version:     "v0.1.0",
		CommandPath: "wheel review post",
		Args:        RedactArgs([]string{"--body", "secret"}),
		GoOS:        "darwin",
		GoArch:      "arm64",
		GoVersion:   "go1.26.2",
		GHVersion:   "2.50.0",
		ErrCode:     string(cliexit.ErrCodeGeneral),
		ErrMessage:  "something broke",
	}
	title, body := BuildIssue(c)

	if !strings.HasPrefix(title, "[auto-report]") {
		t.Errorf("title prefix: got %q", title)
	}
	if !strings.Contains(title, "wheel review post") {
		t.Errorf("title should contain command path: %q", title)
	}
	for _, want := range []string{"wheel review post", "--body " + redactedValue, "v0.1.0", "something broke", "darwin/arm64"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "secret") {
		t.Errorf("body leaked unmasked value:\n%s", body)
	}
}

func TestBuildIssue_PanicIncludesStackAndScrubsToken(t *testing.T) {
	c := Context{
		CommandPath: "wheel task",
		ErrCode:     "PANIC",
		ErrMessage:  "leaked token ghp_abcdefghij0123456789ABCDEFGHIJ in message",
		Stack:       "goroutine 1 [running]:\nmain.main()",
		IsPanic:     true,
	}
	_, body := BuildIssue(c)

	if !strings.Contains(body, "スタックトレース") || !strings.Contains(body, "goroutine 1") {
		t.Errorf("panic body should include stack trace:\n%s", body)
	}
	if strings.Contains(body, "ghp_abcdefghij0123456789ABCDEFGHIJ") {
		t.Errorf("token not scrubbed:\n%s", body)
	}
	if !strings.Contains(body, redactedToken) {
		t.Errorf("expected token placeholder %q in body:\n%s", redactedToken, body)
	}
}

func TestSignature(t *testing.T) {
	c := Context{CommandPath: "wheel graph", ErrCode: "INTERNAL_ERROR"}
	if got, want := c.Signature(), "INTERNAL_ERROR in wheel graph"; got != want {
		t.Errorf("Signature() = %q, want %q", got, want)
	}
	empty := Context{}
	if got, want := empty.Signature(), "PANIC in wheel"; got != want {
		t.Errorf("Signature() fallback = %q, want %q", got, want)
	}
}

func newTestClient(t *testing.T, srv *httptest.Server) *ghclient.Client {
	t.Helper()
	transport := &ghclient.TestTransport{Inner: srv.Client().Transport, BaseURL: srv.URL}
	c, err := ghclient.NewForTest(transport, ReportOwner, ReportRepo)
	if err != nil {
		t.Fatalf("newTestClient: %v", err)
	}
	return c
}

func TestFindExisting(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/search/issues", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"html_url": "https://github.com/UtakataKyosui/gh-wheel/issues/100"},
			},
		})
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)

	got, err := findExisting(newTestClient(t, srv), "INTERNAL_ERROR in wheel review post")
	if err != nil {
		t.Fatalf("findExisting: %v", err)
	}
	if got != "https://github.com/UtakataKyosui/gh-wheel/issues/100" {
		t.Errorf("got %q", got)
	}
}

func TestFindExisting_None(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/search/issues", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{}})
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)

	got, err := findExisting(newTestClient(t, srv), "X in wheel")
	if err != nil {
		t.Fatalf("findExisting: %v", err)
	}
	if got != "" {
		t.Errorf("expected no match, got %q", got)
	}
}

func TestSubmit(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/UtakataKyosui/gh-wheel/issues", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		labels, _ := body["labels"].([]any)
		if len(labels) != 1 || labels[0] != ReportLabel {
			t.Errorf("labels: got %v, want [%s]", body["labels"], ReportLabel)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"html_url": "https://github.com/UtakataKyosui/gh-wheel/issues/200",
		})
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)

	got, err := submit(newTestClient(t, srv), "[auto-report] X in wheel", "body")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if got != "https://github.com/UtakataKyosui/gh-wheel/issues/200" {
		t.Errorf("got %q", got)
	}
}

func TestConfirm(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"y\n", true},
		{"yes\n", true},
		{"Y\n", true},
		{"\n", false},
		{"n\n", false},
		{"nope\n", false},
	}
	for _, tc := range cases {
		var out strings.Builder
		if got := confirm(strings.NewReader(tc.in), &out); got != tc.want {
			t.Errorf("confirm(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
