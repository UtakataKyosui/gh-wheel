package skill

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// rootWithSkill builds a minimal root with the skill subcommand attached so
// cmd.Root() introspection works during Execute.
func rootWithSkill() *cobra.Command {
	root := &cobra.Command{
		Use:           "wheel",
		Short:         "A unified gh extension for Issue-Driven development",
		Long:          "long description",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.PersistentFlags().StringP("repo", "R", "", "Repository in owner/repo format")
	root.AddCommand(NewCmd())
	return root
}

func TestSkill_Stdout(t *testing.T) {
	root := rootWithSkill()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"skill"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := out.String()
	if !strings.HasPrefix(got, "---\n") {
		t.Errorf("expected SKILL.md on stdout, got:\n%s", got)
	}
	if !strings.Contains(got, "name: gh-wheel") {
		t.Errorf("missing default frontmatter name:\n%s", got)
	}
}

func TestSkill_OutputFile(t *testing.T) {
	dir := t.TempDir()
	root := rootWithSkill()
	var errBuf bytes.Buffer
	root.SetErr(&errBuf)
	root.SetArgs([]string{"skill", "--output", dir, "--name", "myskill"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	path := filepath.Join(dir, "myskill", "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	if !strings.Contains(string(data), "name: myskill") {
		t.Errorf("generated file missing 'name: myskill':\n%s", data)
	}
	if !strings.Contains(errBuf.String(), path) {
		t.Errorf("stderr should report the written path, got: %q", errBuf.String())
	}
}

func TestSkill_InvalidName(t *testing.T) {
	for _, bad := range []string{"../evil", "a/b", "Bad Name", ".."} {
		t.Run(bad, func(t *testing.T) {
			root := rootWithSkill()
			root.SetOut(&bytes.Buffer{})
			root.SetErr(&bytes.Buffer{})
			root.SetArgs([]string{"skill", "--name", bad})
			if err := root.Execute(); err == nil {
				t.Errorf("expected error for invalid name %q", bad)
			}
		})
	}
}
