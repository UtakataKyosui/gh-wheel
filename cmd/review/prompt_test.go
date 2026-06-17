package review

import (
	"bytes"
	"strings"
	"testing"
)

func TestBuildDiff_UnderLimit(t *testing.T) {
	files := []prFile{
		{Filename: "main.go", Patch: "@@ -1,3 +1,4 @@\n+added line\n context\n"},
		{Filename: "README.md", Patch: "@@ -1 +1 @@\n-old\n+new\n"},
	}
	got := buildDiff(files)
	if !strings.Contains(got, "--- a/main.go") {
		t.Errorf("expected '--- a/main.go' header, got:\n%s", got)
	}
	if !strings.Contains(got, "+++ b/main.go") {
		t.Errorf("expected '+++ b/main.go' header, got:\n%s", got)
	}
	if !strings.Contains(got, "@@ -1,3 +1,4 @@") {
		t.Errorf("expected hunk header in output, got:\n%s", got)
	}
	if !strings.Contains(got, "--- a/README.md") {
		t.Errorf("expected '--- a/README.md' header, got:\n%s", got)
	}
	if strings.Contains(got, "[diff truncated") {
		t.Errorf("unexpected truncation marker in short diff:\n%s", got)
	}
}

func TestBuildDiff_Truncated(t *testing.T) {
	patch := strings.Repeat("x", diffMaxBytes+100)
	files := []prFile{{Filename: "big.go", Patch: patch}}
	got := buildDiff(files)
	if !strings.Contains(got, "truncated") {
		t.Errorf("expected truncation marker for oversized diff")
	}
	if len(got) > diffMaxBytes+200 {
		t.Errorf("truncated diff too long: %d bytes", len(got))
	}
}

func TestBuildDiff_NoPatch(t *testing.T) {
	files := []prFile{{Filename: "binary.png", Patch: ""}}
	got := buildDiff(files)
	if !strings.Contains(got, "--- a/binary.png") {
		t.Errorf("expected file header even without patch")
	}
}

func TestRenderPrompt_ContainsFields(t *testing.T) {
	meta := prMeta{Number: 42, ChangedFiles: 3}
	meta.Title = "Fix the bug"
	meta.User.Login = "alice"

	var buf bytes.Buffer
	renderPrompt(&buf, meta, "--- a/foo.go\n+++ b/foo.go\n+fix\n")
	out := buf.String()

	checks := []string{
		"# PR Review Request",
		"**PR**: #42",
		"Fix the bug",
		"@alice",
		"**Changed files**: 3",
		"```diff",
		"--- a/foo.go",
		"## Review Schema",
		"```json",
		"## Review Rules",
		"suggestion",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("renderPrompt output missing %q", want)
		}
	}
}
