package graph

import (
	"encoding/json"
	"strings"
	"testing"

	model "github.com/UtakataKyosui/gh-wheel/internal/graph"
)

func TestNewCmd_Use(t *testing.T) {
	cmd := NewCmd()
	if cmd.Use != "graph" {
		t.Errorf("expected Use %q, got %q", "graph", cmd.Use)
	}
}

func TestNewCmd_HasDescription(t *testing.T) {
	cmd := NewCmd()
	if cmd.Short == "" {
		t.Error("Short description must not be empty")
	}
	if cmd.Long == "" {
		t.Error("Long description must not be empty")
	}
}

func TestNewCmd_Flags(t *testing.T) {
	cmd := NewCmd()

	flags := []string{"issue", "depth", "label", "milestone", "no-timeline", "no-sub-issues", "jq", "format"}
	for _, name := range flags {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not registered", name)
		}
	}
}

// buildTestGraph returns a small but representative graph used by formatter tests.
//
// Graph:
//   PR #5 -[closes]-> Issue #1
//   Issue #1 -[sub_issue]-> Issue #2
func buildTestGraph() *model.Graph {
	g := model.NewGraph()
	g.AddNode(&model.Node{
		ID:     "pr_5",
		Number: 5,
		Kind:   model.NodeKindPR,
		State:  "OPEN",
		Title:  "Fix the bug",
		URL:    "https://github.com/owner/repo/pull/5",
	})
	g.AddNode(&model.Node{
		ID:     "issue_1",
		Number: 1,
		Kind:   model.NodeKindIssue,
		State:  "OPEN",
		Title:  "Root issue",
		URL:    "https://github.com/owner/repo/issues/1",
		Labels: []string{"bug"},
	})
	g.AddNode(&model.Node{
		ID:     "issue_2",
		Number: 2,
		Kind:   model.NodeKindIssue,
		State:  "CLOSED",
		Title:  "Sub issue",
		URL:    "https://github.com/owner/repo/issues/2",
	})
	g.AddEdge(model.Edge{Source: "pr_5", Target: "issue_1", Type: model.EdgeCloses})
	g.AddEdge(model.Edge{Source: "issue_1", Target: "issue_2", Type: model.EdgeSubIssue})
	return g
}

// TestFormatList verifies that list output contains one line per node.
func TestFormatList(t *testing.T) {
	g := buildTestGraph()
	out := formatList(g)

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Errorf("formatList: want 3 lines, got %d\n%s", len(lines), out)
	}

	// Each line should mention the issue/PR number
	for _, line := range lines {
		if !strings.Contains(line, "#") {
			t.Errorf("formatList: line missing '#': %q", line)
		}
	}
}

// TestFormatDot verifies DOT output has a valid digraph structure.
func TestFormatDot(t *testing.T) {
	g := buildTestGraph()
	out := formatDot(g)

	if !strings.HasPrefix(strings.TrimSpace(out), "digraph G {") {
		t.Errorf("formatDot: output does not start with 'digraph G {'\n%s", out)
	}
	if !strings.HasSuffix(strings.TrimSpace(out), "}") {
		t.Errorf("formatDot: output does not end with '}'\n%s", out)
	}
	// Should have edge arrow syntax
	if !strings.Contains(out, "->") {
		t.Errorf("formatDot: output should contain '->' for edges\n%s", out)
	}
	// Should have at least node label entries
	if !strings.Contains(out, "label=") {
		t.Errorf("formatDot: output should contain 'label=' for node attributes\n%s", out)
	}
}

// TestFormatTree verifies tree output has indented structure.
func TestFormatTree(t *testing.T) {
	g := buildTestGraph()
	out := formatTree(g)

	if out == "" {
		t.Error("formatTree: output should not be empty")
	}
	// Some nodes should be indented (child nodes)
	if !strings.Contains(out, "  ") {
		t.Errorf("formatTree: output should have indented lines\n%s", out)
	}
}

// TestFormatJSON verifies JSON output is valid and contains expected fields.
func TestFormatJSON(t *testing.T) {
	g := buildTestGraph()
	out, err := formatJSON(g)
	if err != nil {
		t.Fatalf("formatJSON error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("formatJSON: invalid JSON: %v\n%s", err, out)
	}

	if _, ok := parsed["nodes"]; !ok {
		t.Error("formatJSON: 'nodes' key missing")
	}
	if _, ok := parsed["edges"]; !ok {
		t.Error("formatJSON: 'edges' key missing")
	}
	if _, ok := parsed["stats"]; !ok {
		t.Error("formatJSON: 'stats' key missing")
	}
}
