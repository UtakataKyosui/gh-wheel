package main

import (
	"errors"
	"testing"

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
