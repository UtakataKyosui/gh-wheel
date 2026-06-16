package task

import (
	"strings"
	"testing"
)

func TestBuildDiff_Normal(t *testing.T) {
	files := []prFile{
		{Filename: "foo.go", Patch: "@@ -1,3 +1,4 @@\n context\n+added\n context\n context"},
		{Filename: "bar.go", Patch: "@@ -5,2 +5,2 @@\n-old\n+new"},
	}
	got := buildDiff(files)

	if !strings.Contains(got, "--- a/foo.go") {
		t.Errorf("expected '--- a/foo.go' header, got:\n%s", got)
	}
	if !strings.Contains(got, "+++ b/foo.go") {
		t.Errorf("expected '+++ b/foo.go' header, got:\n%s", got)
	}
	if !strings.Contains(got, "--- a/bar.go") {
		t.Errorf("expected '--- a/bar.go' header, got:\n%s", got)
	}
	if !strings.Contains(got, "@@ -1,3 +1,4 @@") {
		t.Errorf("expected first hunk header in output, got:\n%s", got)
	}
	if !strings.Contains(got, "@@ -5,2 +5,2 @@") {
		t.Errorf("expected second hunk header in output, got:\n%s", got)
	}
	// No truncation marker for short diffs.
	if strings.Contains(got, "[diff truncated") {
		t.Errorf("unexpected truncation marker in short diff:\n%s", got)
	}
}

func TestBuildDiff_Truncation(t *testing.T) {
	// Build a patch that will exceed 80000 bytes when assembled.
	bigPatch := strings.Repeat("+line\n", 20000) // ~120000 bytes
	files := []prFile{
		{Filename: "huge.go", Patch: bigPatch},
	}
	got := buildDiff(files)

	if len(got) > 80100 { // allow small overshoot for the marker itself
		t.Errorf("diff not truncated: len=%d", len(got))
	}
	if !strings.Contains(got, "[diff truncated") {
		t.Errorf("truncation marker missing; got len=%d", len(got))
	}
}

func TestBuildDiff_EmptyPatch(t *testing.T) {
	files := []prFile{
		{Filename: "binary.png", Patch: ""},
	}
	got := buildDiff(files)
	// File header should appear even without a patch.
	if !strings.Contains(got, "--- a/binary.png") {
		t.Errorf("expected file header for empty-patch file, got:\n%s", got)
	}
}

func TestRenderPrompt(t *testing.T) {
	meta := prMeta{
		Number:       42,
		Title:        "Fix the thing",
		ChangedFiles: 3,
	}
	meta.User.Login = "octocat"

	var sb strings.Builder
	renderPrompt(&sb, meta, "--- a/x.go\n+++ b/x.go\n@@ -1 +1 @@\n-old\n+new")

	out := sb.String()

	checks := []string{
		"# PR Review Request",
		"**PR**: #42",
		"Fix the thing",
		"@octocat",
		"**Changed files**: 3",
		"```diff",
		"--- a/x.go",
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
