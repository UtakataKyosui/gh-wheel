package main

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"

	"github.com/UtakataKyosui/gh-wheel/internal/cliexit"
)

func TestCobraUsageErr(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
		wantNil  bool
	}{
		{name: "nil", err: nil, wantNil: true},
		{name: "already structured", err: cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs, errors.New("bad")), wantCode: cliexit.CodeUsage},
		{name: "unknown command", err: errors.New("unknown command \"foo\""), wantCode: cliexit.CodeUsage},
		{name: "unknown flag", err: errors.New("unknown flag: --bar"), wantCode: cliexit.CodeUsage},
		{name: "unknown shorthand flag", err: errors.New("unknown shorthand flag: 'x'"), wantCode: cliexit.CodeUsage},
		{name: "accepts", err: errors.New("accepts 1 arg(s), received 2"), wantCode: cliexit.CodeUsage},
		{name: "required flag", err: errors.New("required flag(s) \"file\" not set"), wantCode: cliexit.CodeUsage},
		{name: "invalid argument", err: errors.New("invalid argument \"bogus\" for \"--state\""), wantCode: cliexit.CodeUsage},
		{name: "flag needs an argument", err: errors.New("flag needs an argument: '--file'"), wantCode: cliexit.CodeUsage},
		{name: "general error", err: errors.New("something went wrong"), wantCode: cliexit.CodeGeneral},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cobraUsageErr(tt.err)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil error")
			}
			if code := cliexit.ExitCodeOf(got); code != tt.wantCode {
				t.Errorf("exit code = %d, want %d (err: %v)", code, tt.wantCode, got)
			}
		})
	}
}

func TestOptedOut(t *testing.T) {
	tests := []struct {
		name     string
		noReport bool
		env      string
		want     bool
	}{
		{"neither", false, "", false},
		{"flag only", true, "", true},
		{"env only", false, "1", true},
		{"both", true, "1", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := optedOut(tt.noReport, tt.env); got != tt.want {
				t.Errorf("optedOut(%v, %q) = %v, want %v", tt.noReport, tt.env, got, tt.want)
			}
		})
	}
}

func TestCommandContext(t *testing.T) {
	newRoot := func() *cobra.Command {
		root := &cobra.Command{Use: "wheel"}
		review := &cobra.Command{Use: "review"}
		post := &cobra.Command{Use: "post", RunE: func(*cobra.Command, []string) error { return nil }}
		post.Flags().String("body", "", "")
		review.AddCommand(post)
		root.AddCommand(review)
		return root
	}

	t.Run("leaf command resolved with remaining args", func(t *testing.T) {
		path, rest := commandContext(newRoot(), []string{"gh-wheel", "review", "post", "--body", "x"})
		if path != "wheel review post" {
			t.Errorf("path = %q, want %q", path, "wheel review post")
		}
		if len(rest) != 2 || rest[0] != "--body" || rest[1] != "x" {
			t.Errorf("rest = %v, want [--body x]", rest)
		}
	})

	t.Run("empty argv does not panic", func(t *testing.T) {
		path, rest := commandContext(newRoot(), nil)
		if path != "wheel" {
			t.Errorf("path = %q, want %q", path, "wheel")
		}
		if rest != nil {
			t.Errorf("rest = %v, want nil", rest)
		}
	})

	t.Run("program name only yields root path", func(t *testing.T) {
		path, _ := commandContext(newRoot(), []string{"gh-wheel"})
		if path != "wheel" {
			t.Errorf("path = %q, want %q", path, "wheel")
		}
	})
}
