package cmd

import (
	"testing"
)

func TestNewRootCmd_Use(t *testing.T) {
	root := NewRootCmd()
	if root.Use != "wheel" {
		t.Errorf("expected Use %q, got %q", "wheel", root.Use)
	}
}

func TestNewRootCmd_SilenceFlags(t *testing.T) {
	root := NewRootCmd()
	if !root.SilenceErrors {
		t.Error("SilenceErrors should be true so main controls error rendering")
	}
	if !root.SilenceUsage {
		t.Error("SilenceUsage should be true so main controls usage output")
	}
}

func TestNewRootCmd_PersistentFlags(t *testing.T) {
	root := NewRootCmd()
	flags := []string{"repo", "json", "dry-run", "jq"}
	for _, f := range flags {
		if root.PersistentFlags().Lookup(f) == nil {
			t.Errorf("persistent flag --%s not registered", f)
		}
	}
}

func TestNewRootCmd_Subcommands(t *testing.T) {
	root := NewRootCmd()
	got := make(map[string]bool)
	for _, c := range root.Commands() {
		got[c.Use] = true
	}
	for _, name := range []string{"task", "graph", "monitor", "review", "okr"} {
		if !got[name] {
			t.Errorf("subcommand %q not registered", name)
		}
	}
}
