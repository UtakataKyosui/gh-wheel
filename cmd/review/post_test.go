package review

import (
	"encoding/json"
	"strings"
	"testing"
)

// --- TestBuildPayload_WithSuggestion ---

func TestBuildPayload_WithSuggestion(t *testing.T) {
	doc := ReviewDoc{
		Event:   "COMMENT",
		Summary: "looks good",
		Comments: []ReviewComment{
			{
				Path:       "main.go",
				Body:       "consider renaming",
				Suggestion: "```go\nfmt.Println(\"hello\")\n```",
				Line:       10,
				Side:       "RIGHT",
			},
		},
	}

	payload := buildReviewPayload(doc)

	if payload.Event != "COMMENT" {
		t.Errorf("expected Event=COMMENT, got %q", payload.Event)
	}
	if payload.Body != "looks good" {
		t.Errorf("expected Body=%q, got %q", "looks good", payload.Body)
	}
	if len(payload.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(payload.Comments))
	}

	body := payload.Comments[0].Body
	if !strings.Contains(body, "consider renaming") {
		t.Errorf("comment body should contain original body, got: %q", body)
	}
	if !strings.Contains(body, "```suggestion") {
		t.Errorf("comment body should contain suggestion fence, got: %q", body)
	}
	if !strings.Contains(body, "fmt.Println") {
		t.Errorf("comment body should contain suggestion content, got: %q", body)
	}
}

// --- TestBuildPayload_WithSkipReason ---

func TestBuildPayload_WithSkipReason(t *testing.T) {
	doc := ReviewDoc{
		Event:   "REQUEST_CHANGES",
		Summary: "needs work",
		Comments: []ReviewComment{
			{
				Path:           "util.go",
				Body:           "this needs improvement",
				SkipSuggestion: true,
				Reason:         "complex refactor needed",
				Line:           5,
			},
		},
	}

	payload := buildReviewPayload(doc)

	if len(payload.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(payload.Comments))
	}

	body := payload.Comments[0].Body
	if !strings.Contains(body, "this needs improvement") {
		t.Errorf("comment body should contain original body, got: %q", body)
	}
	if !strings.Contains(body, "<!-- skip-reason: complex refactor needed -->") {
		t.Errorf("comment body should contain skip-reason HTML comment, got: %q", body)
	}
}

// --- TestBuildPayload_StartLineOmitted ---

func TestBuildPayload_StartLineOmitted_WhenZero(t *testing.T) {
	doc := ReviewDoc{
		Event:   "COMMENT",
		Summary: "summary",
		Comments: []ReviewComment{
			{
				Path:       "a.go",
				Body:       "body",
				Suggestion: "```go\ncode\n```",
				Line:       8,
				StartLine:  0,
			},
		},
	}

	payload := buildReviewPayload(doc)
	if payload.Comments[0].StartLine != 0 {
		t.Errorf("expected StartLine=0 when not set, got %d", payload.Comments[0].StartLine)
	}
}

func TestBuildPayload_StartLineOmitted_WhenGeqLine(t *testing.T) {
	doc := ReviewDoc{
		Event:   "COMMENT",
		Summary: "summary",
		Comments: []ReviewComment{
			{
				Path:       "a.go",
				Body:       "body",
				Suggestion: "```go\ncode\n```",
				Line:       5,
				StartLine:  5, // equal to Line, should be omitted
			},
		},
	}

	payload := buildReviewPayload(doc)
	if payload.Comments[0].StartLine != 0 {
		t.Errorf("expected StartLine=0 when start_line >= line, got %d", payload.Comments[0].StartLine)
	}
}

func TestBuildPayload_StartLineOmitted_WhenGreaterThanLine(t *testing.T) {
	doc := ReviewDoc{
		Event:   "COMMENT",
		Summary: "summary",
		Comments: []ReviewComment{
			{
				Path:       "a.go",
				Body:       "body",
				Suggestion: "```go\ncode\n```",
				Line:       5,
				StartLine:  10, // greater than Line, should be omitted
			},
		},
	}

	payload := buildReviewPayload(doc)
	if payload.Comments[0].StartLine != 0 {
		t.Errorf("expected StartLine=0 when start_line > line, got %d", payload.Comments[0].StartLine)
	}
}

func TestBuildPayload_StartLineIncluded_WhenLessThanLine(t *testing.T) {
	doc := ReviewDoc{
		Event:   "COMMENT",
		Summary: "summary",
		Comments: []ReviewComment{
			{
				Path:       "a.go",
				Body:       "body",
				Suggestion: "```go\ncode\n```",
				Line:       10,
				StartLine:  7, // less than Line, should be included
			},
		},
	}

	payload := buildReviewPayload(doc)
	if payload.Comments[0].StartLine != 7 {
		t.Errorf("expected StartLine=7, got %d", payload.Comments[0].StartLine)
	}
}

// --- TestBuildPayload_DefaultSide ---

func TestBuildPayload_DefaultSide_WhenEmpty(t *testing.T) {
	doc := ReviewDoc{
		Event:   "COMMENT",
		Summary: "summary",
		Comments: []ReviewComment{
			{
				Path:       "a.go",
				Body:       "body",
				Suggestion: "```go\ncode\n```",
				Line:       3,
				Side:       "", // should default to RIGHT
			},
		},
	}

	payload := buildReviewPayload(doc)
	if payload.Comments[0].Side != "RIGHT" {
		t.Errorf("expected Side=RIGHT as default, got %q", payload.Comments[0].Side)
	}
}

func TestBuildPayload_SidePreserved(t *testing.T) {
	doc := ReviewDoc{
		Event:   "COMMENT",
		Summary: "summary",
		Comments: []ReviewComment{
			{
				Path:       "a.go",
				Body:       "body",
				Suggestion: "```go\ncode\n```",
				Line:       3,
				Side:       "LEFT",
			},
		},
	}

	payload := buildReviewPayload(doc)
	if payload.Comments[0].Side != "LEFT" {
		t.Errorf("expected Side=LEFT, got %q", payload.Comments[0].Side)
	}
}

// --- TestBuildPayload_DefaultEvent ---

func TestBuildPayload_DefaultEvent_WhenEmpty(t *testing.T) {
	doc := ReviewDoc{
		Event:   "", // empty → default to COMMENT
		Summary: "summary",
		Comments: []ReviewComment{
			{Path: "a.go", Body: "body", Suggestion: "```go\ncode\n```", Line: 1},
		},
	}

	payload := buildReviewPayload(doc)
	if payload.Event != "COMMENT" {
		t.Errorf("expected Event=COMMENT as default, got %q", payload.Event)
	}
}

// --- TestPost_DryRun ---

func TestPost_DryRun_OutputsJSON(t *testing.T) {
	// Build a payload and verify it serializes to valid JSON with expected fields.
	doc := ReviewDoc{
		Event:   "COMMENT",
		Summary: "dry run summary",
		Comments: []ReviewComment{
			{
				Path:       "main.go",
				Body:       "consider this",
				Suggestion: "```go\nfmt.Println()\n```",
				Line:       42,
			},
		},
	}

	payload := buildReviewPayload(doc)

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent failed: %v", err)
	}

	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"event"`) {
		t.Error("JSON should contain 'event' field")
	}
	if !strings.Contains(jsonStr, `"body"`) {
		t.Error("JSON should contain 'body' field")
	}
	if !strings.Contains(jsonStr, `"comments"`) {
		t.Error("JSON should contain 'comments' field")
	}
	if !strings.Contains(jsonStr, "dry run summary") {
		t.Error("JSON should contain the summary text")
	}
}

func TestPost_DryRun_StartLineOmittedInJSON(t *testing.T) {
	// When StartLine is 0, it should be omitted in JSON (omitempty).
	doc := ReviewDoc{
		Event:   "COMMENT",
		Summary: "summary",
		Comments: []ReviewComment{
			{
				Path:       "a.go",
				Body:       "body",
				Suggestion: "```go\ncode\n```",
				Line:       5,
				StartLine:  0,
			},
		},
	}

	payload := buildReviewPayload(doc)
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	if strings.Contains(string(data), "start_line") {
		t.Error("JSON should not contain 'start_line' when StartLine=0 (omitempty)")
	}
}
