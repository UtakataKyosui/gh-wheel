package okr

import "testing"

func TestNewCmd(t *testing.T) {
	cmd := NewCmd()
	if cmd.Use != "okr" {
		t.Errorf("Use = %q, want %q", cmd.Use, "okr")
	}
	if cmd.Short == "" || cmd.Long == "" {
		t.Error("Short and Long must be non-empty")
	}
}

func TestNewCmd_HasMetricsSubcommand(t *testing.T) {
	cmd := NewCmd()
	var found bool
	for _, sub := range cmd.Commands() {
		if sub.Name() == "metrics" {
			found = true
			break
		}
	}
	if !found {
		t.Error("okr command should register the metrics subcommand")
	}
}
