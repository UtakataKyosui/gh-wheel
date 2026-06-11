package auth_test

import (
	"testing"

	"github.com/UtakataKyosui/gh-wheel/internal/auth"
	"github.com/UtakataKyosui/gh-wheel/internal/cliexit"
)

func TestParseGHVersion_Valid(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"exact minimum", "gh version 2.40.0 (2024-01-01)\nhttps://github.com/cli/cli/releases/tag/v2.40.0\n"},
		{"minor above", "gh version 2.41.0 (2024-01-01)\n"},
		{"higher major", "gh version 3.0.0 (2024-01-01)\n"},
		{"patch variation", "gh version 2.40.9 (2024-01-01)\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := auth.ParseGHVersion(tc.input); err != nil {
				t.Errorf("ParseGHVersion(%q) = %v, want nil", tc.input, err)
			}
		})
	}
}

func TestParseGHVersion_TooOld(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"minor too low", "gh version 2.39.9 (2024-01-01)\n"},
		{"major 1", "gh version 1.99.99 (2024-01-01)\n"},
		{"minor 0", "gh version 2.0.0 (2024-01-01)\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := auth.ParseGHVersion(tc.input)
			if err == nil {
				t.Fatalf("ParseGHVersion(%q) = nil, want error", tc.input)
			}
			if code := cliexit.ExitCodeOf(err); code != cliexit.CodeAuth {
				t.Errorf("exit code = %d, want CodeAuth (%d)", code, cliexit.CodeAuth)
			}
		})
	}
}

func TestParseGHVersion_Unparseable(t *testing.T) {
	err := auth.ParseGHVersion("some unexpected output")
	if err == nil {
		t.Fatal("expected error for unparseable output, got nil")
	}
	if code := cliexit.ExitCodeOf(err); code != cliexit.CodeAuth {
		t.Errorf("exit code = %d, want CodeAuth (%d)", code, cliexit.CodeAuth)
	}
}
