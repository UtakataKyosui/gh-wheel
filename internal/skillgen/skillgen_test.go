package skillgen

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func noop(_ *cobra.Command, _ []string) error { return nil }

// sampleRoot builds a small cobra tree resembling gh-wheel's structure.
func sampleRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "wheel",
		Short: "A unified gh extension for Issue-Driven development",
		Long:  "gh-wheel integrates task management, graphs, and review workflows.",
	}
	root.PersistentFlags().StringP("repo", "R", "", "Repository in owner/repo format")
	root.PersistentFlags().BoolP("json", "j", false, "Output results as JSON")
	root.PersistentFlags().Bool("no-report", false, "Do not offer to file an issue on unexpected errors")

	task := &cobra.Command{Use: "task", Short: "Manage tasks", RunE: noop}
	task.Flags().StringP("state", "s", "open", "Filter by state: open, closed, all")
	closeCmd := &cobra.Command{Use: "close <N>", Short: "Close a PR or Issue", RunE: noop}
	task.AddCommand(closeCmd)

	review := &cobra.Command{Use: "review", Short: "AI-assisted review workflows", RunE: noop}

	root.AddCommand(task, review)
	return root
}

func TestGenerate_Frontmatter(t *testing.T) {
	out := Generate(sampleRoot(), Options{})
	if !strings.HasPrefix(out, "---\n") {
		t.Fatalf("output should start with YAML frontmatter, got:\n%q", out)
	}
	if !strings.Contains(out, "name: gh-wheel\n") {
		t.Errorf("default name missing:\n%s", out)
	}
	if !strings.Contains(out, "description:") {
		t.Errorf("description missing:\n%s", out)
	}
	// Frontmatter must be closed.
	if strings.Count(out, "---\n") < 2 {
		t.Errorf("frontmatter not closed:\n%s", out)
	}
}

func TestGenerate_NameAndDescriptionOverride(t *testing.T) {
	out := Generate(sampleRoot(), Options{Name: "wheelctl", Description: "custom desc"})
	if !strings.Contains(out, "name: wheelctl\n") {
		t.Errorf("name override missing:\n%s", out)
	}
	if !strings.Contains(out, "description: custom desc\n") {
		t.Errorf("description override missing:\n%s", out)
	}
}

func TestGenerate_DescriptionSingleLine(t *testing.T) {
	out := Generate(sampleRoot(), Options{Description: "line one\nline two"})
	if !strings.Contains(out, "description: line one line two\n") {
		t.Errorf("newlines in description should be flattened:\n%s", out)
	}
}

func TestGenerate_CommandsAndFlags(t *testing.T) {
	out := Generate(sampleRoot(), Options{})
	for _, want := range []string{
		"gh wheel task",
		"gh wheel task close",
		"gh wheel review",
		"--state",     // command-local flag documented
		"## グローバルフラグ", // global flag section
		"--repo",
		"--json",
		"## 補足", // notes section
		"auto-report",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestGenerate_AutoReportNoteConditional(t *testing.T) {
	// With a no-report flag present, the auto-report note appears.
	withFlag := Generate(sampleRoot(), Options{})
	if !strings.Contains(withFlag, "auto-report") {
		t.Errorf("auto-report note should appear when --no-report exists")
	}

	// Without the flag (feature absent), the note must not be emitted.
	root := &cobra.Command{Use: "wheel", Short: "x"}
	root.PersistentFlags().BoolP("json", "j", false, "json")
	root.AddCommand(&cobra.Command{Use: "task", Short: "t", RunE: noop})
	if got := Generate(root, Options{}); strings.Contains(got, "auto-report") {
		t.Errorf("auto-report note should be omitted when --no-report is absent:\n%s", got)
	}
}

func TestGenerate_YAMLSensitiveDescription(t *testing.T) {
	// A description with YAML-sensitive characters must stay valid (quoted).
	out := Generate(sampleRoot(), Options{Description: "fixes: #42 and # more"})
	var fm struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	// Extract the frontmatter block between the first two "---" lines.
	parts := strings.SplitN(out, "---\n", 3)
	if len(parts) < 3 {
		t.Fatalf("missing frontmatter block:\n%s", out)
	}
	if err := yaml.Unmarshal([]byte(parts[1]), &fm); err != nil {
		t.Fatalf("frontmatter is not valid YAML: %v\n%s", err, parts[1])
	}
	if fm.Description != "fixes: #42 and # more" {
		t.Errorf("description round-trip mismatch: %q", fm.Description)
	}
}

func TestGenerate_SkipsBuiltins(t *testing.T) {
	out := Generate(sampleRoot(), Options{})
	if strings.Contains(out, "gh wheel help") {
		t.Errorf("help command should be skipped:\n%s", out)
	}
	if strings.Contains(out, "gh wheel completion") {
		t.Errorf("completion command should be skipped:\n%s", out)
	}
}
