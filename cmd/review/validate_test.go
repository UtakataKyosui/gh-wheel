package review

import (
	"testing"
)

// --- helper ---
func noErr(t *testing.T, warnings, errs []string) {
	t.Helper()
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func hasErr(t *testing.T, errs []string, substr string) {
	t.Helper()
	for _, e := range errs {
		if contains(e, substr) {
			return
		}
	}
	t.Fatalf("expected an error containing %q, got: %v", substr, errs)
}

func hasWarn(t *testing.T, warnings []string, substr string) {
	t.Helper()
	for _, w := range warnings {
		if contains(w, substr) {
			return
		}
	}
	t.Fatalf("expected a warning containing %q, got: %v", substr, warnings)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}

// --- TestValidate_RequiredFields ---

func TestValidate_RequiredFields_MissingEvent(t *testing.T) {
	doc := ReviewDoc{
		Event:    "",
		Summary:  "looks good",
		Comments: []ReviewComment{{Path: "a.go", Body: "nice", SkipSuggestion: true, Reason: "no suggestion needed"}},
	}
	_, errs := ValidateDoc(doc, validateOpts{MinComments: 1})
	hasErr(t, errs, "event")
}

func TestValidate_RequiredFields_MissingSummary(t *testing.T) {
	doc := ReviewDoc{
		Event:    "COMMENT",
		Summary:  "",
		Comments: []ReviewComment{{Path: "a.go", Body: "nice", SkipSuggestion: true, Reason: "no suggestion needed"}},
	}
	_, errs := ValidateDoc(doc, validateOpts{MinComments: 1})
	hasErr(t, errs, "summary")
}

func TestValidate_RequiredFields_MissingComments(t *testing.T) {
	doc := ReviewDoc{
		Event:    "COMMENT",
		Summary:  "looks good",
		Comments: nil,
	}
	_, errs := ValidateDoc(doc, validateOpts{MinComments: 0})
	hasErr(t, errs, "comments")
}

func TestValidate_RequiredFields_Valid(t *testing.T) {
	doc := ReviewDoc{
		Event:   "COMMENT",
		Summary: "looks good",
		Comments: []ReviewComment{
			{Path: "a.go", Body: "nice", Suggestion: "```go\nfmt.Println()\n```"},
		},
	}
	warnings, errs := ValidateDoc(doc, validateOpts{MinComments: 1})
	noErr(t, warnings, errs)
}

// --- TestValidate_EventValues ---

func TestValidate_InvalidEvent(t *testing.T) {
	doc := ReviewDoc{
		Event:    "INVALID",
		Summary:  "looks good",
		Comments: []ReviewComment{{Path: "a.go", Body: "ok", SkipSuggestion: true, Reason: "n/a"}},
	}
	_, errs := ValidateDoc(doc, validateOpts{MinComments: 1})
	hasErr(t, errs, "event")
}

func TestValidate_ValidEvents(t *testing.T) {
	for _, ev := range []string{"COMMENT", "REQUEST_CHANGES", "APPROVE"} {
		doc := ReviewDoc{
			Event:   ev,
			Summary: "looks good",
			Comments: []ReviewComment{
				{Path: "a.go", Body: "ok", Suggestion: "```go\nfmt.Println()\n```"},
			},
		}
		_, errs := ValidateDoc(doc, validateOpts{MinComments: 1})
		for _, e := range errs {
			if contains(e, "event") {
				t.Errorf("event=%q: unexpected event error: %s", ev, e)
			}
		}
	}
}

// --- TestValidate_SuggestionInSummary ---

func TestValidate_SuggestionInSummary(t *testing.T) {
	doc := ReviewDoc{
		Event:   "COMMENT",
		Summary: "looks good ```suggestion\nfoo\n```",
		Comments: []ReviewComment{
			{Path: "a.go", Body: "ok", SkipSuggestion: true, Reason: "n/a"},
		},
	}
	_, errs := ValidateDoc(doc, validateOpts{MinComments: 1})
	hasErr(t, errs, "suggestion")
}

func TestValidate_SuggestionNotInSummary_OK(t *testing.T) {
	doc := ReviewDoc{
		Event:   "COMMENT",
		Summary: "looks good",
		Comments: []ReviewComment{
			{Path: "a.go", Body: "ok", Suggestion: "```go\nfmt.Println()\n```"},
		},
	}
	_, errs := ValidateDoc(doc, validateOpts{MinComments: 1})
	for _, e := range errs {
		if contains(e, "suggestion fence in summary") {
			t.Errorf("unexpected suggestion-in-summary error: %s", e)
		}
	}
}

// --- TestValidate_MinComments ---

func TestValidate_MinComments_BelowMin(t *testing.T) {
	doc := ReviewDoc{
		Event:   "COMMENT",
		Summary: "looks good",
		Comments: []ReviewComment{
			{Path: "a.go", Body: "ok", SkipSuggestion: true, Reason: "n/a"},
		},
	}
	_, errs := ValidateDoc(doc, validateOpts{MinComments: 3})
	hasErr(t, errs, "comment")
}

func TestValidate_MinComments_AtMin(t *testing.T) {
	doc := ReviewDoc{
		Event:   "COMMENT",
		Summary: "looks good",
		Comments: []ReviewComment{
			{Path: "a.go", Body: "ok", Suggestion: "```go\nfmt.Println()\n```"},
			{Path: "b.go", Body: "ok", Suggestion: "```go\nfmt.Println()\n```"},
			{Path: "c.go", Body: "ok", Suggestion: "```go\nfmt.Println()\n```"},
		},
	}
	_, errs := ValidateDoc(doc, validateOpts{MinComments: 3})
	for _, e := range errs {
		if contains(e, "comment") {
			t.Errorf("unexpected min-comments error: %s", e)
		}
	}
}

// --- TestValidate_SkipSuggestionMissingReason ---

func TestValidate_SkipSuggestion_NoReason(t *testing.T) {
	doc := ReviewDoc{
		Event:   "COMMENT",
		Summary: "looks good",
		Comments: []ReviewComment{
			{Path: "a.go", Body: "ok", SkipSuggestion: true, Reason: ""},
		},
	}
	_, errs := ValidateDoc(doc, validateOpts{MinComments: 1})
	hasErr(t, errs, "reason")
}

func TestValidate_SkipSuggestion_WithReason_OK(t *testing.T) {
	doc := ReviewDoc{
		Event:   "COMMENT",
		Summary: "looks good",
		Comments: []ReviewComment{
			{Path: "a.go", Body: "ok", SkipSuggestion: true, Reason: "style only"},
		},
	}
	_, errs := ValidateDoc(doc, validateOpts{MinComments: 1})
	noErr(t, nil, errs)
}

func TestValidate_NoSuggestionAndNoSkip(t *testing.T) {
	doc := ReviewDoc{
		Event:   "COMMENT",
		Summary: "looks good",
		Comments: []ReviewComment{
			{Path: "a.go", Body: "ok", Suggestion: "", SkipSuggestion: false, Reason: ""},
		},
	}
	_, errs := ValidateDoc(doc, validateOpts{MinComments: 1})
	hasErr(t, errs, "suggestion")
}

// --- TestValidate_SkipRatio ---

func TestValidate_SkipRatio_Warning(t *testing.T) {
	// 3 of 4 comments are skip (75% > 50%) → warning, not error
	doc := ReviewDoc{
		Event:   "COMMENT",
		Summary: "looks good",
		Comments: []ReviewComment{
			{Path: "a.go", Body: "ok", SkipSuggestion: true, Reason: "r"},
			{Path: "b.go", Body: "ok", SkipSuggestion: true, Reason: "r"},
			{Path: "c.go", Body: "ok", SkipSuggestion: true, Reason: "r"},
			{Path: "d.go", Body: "ok", Suggestion: "```go\nfmt.Println()\n```"},
		},
	}
	warnings, errs := ValidateDoc(doc, validateOpts{MinComments: 1, Strict: false})
	hasWarn(t, warnings, "skip")
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestValidate_SkipRatio_StrictError(t *testing.T) {
	doc := ReviewDoc{
		Event:   "COMMENT",
		Summary: "looks good",
		Comments: []ReviewComment{
			{Path: "a.go", Body: "ok", SkipSuggestion: true, Reason: "r"},
			{Path: "b.go", Body: "ok", SkipSuggestion: true, Reason: "r"},
			{Path: "c.go", Body: "ok", SkipSuggestion: true, Reason: "r"},
			{Path: "d.go", Body: "ok", Suggestion: "```go\nfmt.Println()\n```"},
		},
	}
	_, errs := ValidateDoc(doc, validateOpts{MinComments: 1, Strict: true})
	hasErr(t, errs, "skip")
}

func TestValidate_SkipRatio_AtThreshold_NoWarn(t *testing.T) {
	// 50% exactly is NOT > 50%, so no warning
	doc := ReviewDoc{
		Event:   "COMMENT",
		Summary: "looks good",
		Comments: []ReviewComment{
			{Path: "a.go", Body: "ok", SkipSuggestion: true, Reason: "r"},
			{Path: "b.go", Body: "ok", Suggestion: "```go\nfmt.Println()\n```"},
		},
	}
	warnings, _ := ValidateDoc(doc, validateOpts{MinComments: 1, Strict: false})
	for _, w := range warnings {
		if contains(w, "skip") {
			t.Errorf("unexpected skip ratio warning at exactly 50%%: %s", w)
		}
	}
}

// --- TestValidate_SuggestionFenceValidation ---

func TestValidate_SuggestionMissingLanguageTag(t *testing.T) {
	doc := ReviewDoc{
		Event:   "COMMENT",
		Summary: "looks good",
		Comments: []ReviewComment{
			// Opening fence without language tag
			{Path: "a.go", Body: "ok", Suggestion: "```\nfmt.Println()\n```"},
		},
	}
	_, errs := ValidateDoc(doc, validateOpts{MinComments: 1})
	hasErr(t, errs, "language")
}

func TestValidate_SuggestionWithLanguageTag_OK(t *testing.T) {
	doc := ReviewDoc{
		Event:   "COMMENT",
		Summary: "looks good",
		Comments: []ReviewComment{
			{Path: "a.go", Body: "ok", Suggestion: "```go\nfmt.Println()\n```"},
		},
	}
	_, errs := ValidateDoc(doc, validateOpts{MinComments: 1})
	noErr(t, nil, errs)
}

func TestValidate_SuggestionUnclosedFence(t *testing.T) {
	doc := ReviewDoc{
		Event:   "COMMENT",
		Summary: "looks good",
		Comments: []ReviewComment{
			// Only one ```, fence is not closed
			{Path: "a.go", Body: "ok", Suggestion: "```go\nfmt.Println()"},
		},
	}
	_, errs := ValidateDoc(doc, validateOpts{MinComments: 1})
	hasErr(t, errs, "fence")
}

// --- TestValidate_ParseYAML ---

func TestValidate_ParseYAML(t *testing.T) {
	yamlContent := `event: COMMENT
summary: "looks good"
comments:
  - path: a.go
    body: nice
    suggestion: "` + "```" + `go\nfmt.Println()\n` + "```" + `"
`
	// Write to temp file
	dir := t.TempDir()
	path := dir + "/review.yaml"
	if err := writeFile(path, []byte(yamlContent)); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	doc, err := parseReviewFile(path, "")
	if err != nil {
		t.Fatalf("parseReviewFile error: %v", err)
	}
	if doc.Event != "COMMENT" {
		t.Errorf("expected Event=COMMENT, got %q", doc.Event)
	}
	if doc.Summary != "looks good" {
		t.Errorf("expected Summary=%q, got %q", "looks good", doc.Summary)
	}
	if len(doc.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(doc.Comments))
	}
	if doc.Comments[0].Path != "a.go" {
		t.Errorf("expected comment path a.go, got %q", doc.Comments[0].Path)
	}
}

func TestValidate_ParseJSON(t *testing.T) {
	jsonContent := `{
  "event": "REQUEST_CHANGES",
  "summary": "needs work",
  "comments": [
    {"path": "b.go", "body": "fix this", "skip_suggestion": true, "reason": "style only"}
  ]
}`
	dir := t.TempDir()
	path := dir + "/review.json"
	if err := writeFile(path, []byte(jsonContent)); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	doc, err := parseReviewFile(path, "")
	if err != nil {
		t.Fatalf("parseReviewFile error: %v", err)
	}
	if doc.Event != "REQUEST_CHANGES" {
		t.Errorf("expected Event=REQUEST_CHANGES, got %q", doc.Event)
	}
	if len(doc.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(doc.Comments))
	}
	if !doc.Comments[0].SkipSuggestion {
		t.Error("expected SkipSuggestion=true")
	}
}

func TestValidate_ParseYAMLFormatFlag(t *testing.T) {
	// .txt extension but --format yaml override
	yamlContent := "event: APPROVE\nsummary: all good\ncomments:\n  - path: c.go\n    body: ok\n    skip_suggestion: true\n    reason: simple change\n"
	dir := t.TempDir()
	path := dir + "/review.txt"
	if err := writeFile(path, []byte(yamlContent)); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	doc, err := parseReviewFile(path, "yaml")
	if err != nil {
		t.Fatalf("parseReviewFile with format=yaml: %v", err)
	}
	if doc.Event != "APPROVE" {
		t.Errorf("expected Event=APPROVE, got %q", doc.Event)
	}
}

func TestValidate_ParseUnknownExtension(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/review.txt"
	if err := writeFile(path, []byte("{}")); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	_, err := parseReviewFile(path, "")
	if err == nil {
		t.Fatal("expected error for unknown extension without --format, got nil")
	}
}

// --- TestValidate_SelfApproveGuard ---

func TestValidate_SelfApproveGuard(t *testing.T) {
	doc := ReviewDoc{
		Event:   "APPROVE",
		Summary: "all good",
		Comments: []ReviewComment{
			{Path: "a.go", Body: "ok", SkipSuggestion: true, Reason: "n/a"},
		},
	}
	_, errs := ValidateDoc(doc, validateOpts{MinComments: 1, PRAuthor: "alice", CurrentUser: "alice"})
	hasErr(t, errs, "self-approve")
}

func TestValidate_SelfApproveGuard_DifferentUser_OK(t *testing.T) {
	doc := ReviewDoc{
		Event:   "APPROVE",
		Summary: "all good",
		Comments: []ReviewComment{
			{Path: "a.go", Body: "ok", SkipSuggestion: true, Reason: "n/a"},
		},
	}
	_, errs := ValidateDoc(doc, validateOpts{MinComments: 1, PRAuthor: "alice", CurrentUser: "bob"})
	for _, e := range errs {
		if contains(e, "self-approve") {
			t.Errorf("unexpected self-approve error: %s", e)
		}
	}
}
