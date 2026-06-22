package task

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/UtakataKyosui/gh-wheel/internal/cliexit"
	"github.com/UtakataKyosui/gh-wheel/internal/schedule"
)

func TestSummarize(t *testing.T) {
	r := &TaskResult{PRs: []PR{
		{Number: 1, Categories: []string{"author"}},
		{Number: 2, Categories: []string{"review-requested"}},
		{Number: 3, Categories: []string{"author", "review-requested"}},
	}}
	s := summarize("a/b", r)
	if s.Repo != "a/b" {
		t.Errorf("Repo = %q, want a/b", s.Repo)
	}
	if s.Authored != 2 {
		t.Errorf("Authored = %d, want 2", s.Authored)
	}
	if s.ReviewRequested != 2 {
		t.Errorf("ReviewRequested = %d, want 2", s.ReviewRequested)
	}
}

func TestToView(t *testing.T) {
	last := time.Date(2026, 6, 22, 9, 0, 0, 0, time.UTC)
	v := toView(schedule.Entry{Repo: "a/b", Interval: "30m", LastRun: &last, RunCount: 3})
	if v.Repo != "a/b" || v.RunCount != 3 {
		t.Errorf("view = %+v", v)
	}
	if v.NextRun == nil || !v.NextRun.Equal(last.Add(30*time.Minute)) {
		t.Errorf("NextRun = %v, want %v", v.NextRun, last.Add(30*time.Minute))
	}

	never := toView(schedule.Entry{Repo: "c/d", Interval: "5m"})
	if never.NextRun != nil {
		t.Errorf("NextRun for never-run entry = %v, want nil", never.NextRun)
	}
}

func TestScheduleAddValidation(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"bad state", []string{"--state", "bogus"}},
		{"author and review exclusive", []string{"--author-only", "--review-only"}},
		{"interval too short", []string{"--interval", "10s"}},
		{"interval unparseable", []string{"--interval", "soon"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newScheduleAddCmd()
			cmd.SetArgs(tc.args)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			err := cmd.Execute()
			if err == nil {
				t.Fatalf("expected a usage error for args %v", tc.args)
			}
			var ce *cliexit.Error
			if !errors.As(err, &ce) || ce.ExitCode != cliexit.CodeUsage {
				t.Errorf("error = %v, want *cliexit.Error with exit code %d", err, cliexit.CodeUsage)
			}
		})
	}
}
