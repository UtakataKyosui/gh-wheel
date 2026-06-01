package review

import "testing"

func TestNewCmd_Use(t *testing.T) {
	cmd := NewCmd()
	if cmd.Use != "review" {
		t.Errorf("expected Use %q, got %q", "review", cmd.Use)
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
