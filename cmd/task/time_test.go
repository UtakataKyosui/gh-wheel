package task

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/UtakataKyosui/gh-wheel/internal/worktime"
)

func TestFormatWorkDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0h 0m"},
		{15 * time.Minute, "0h 15m"},
		{30 * time.Minute, "0h 30m"},
		{90 * time.Minute, "1h 30m"},
		{2*time.Hour + 5*time.Minute, "2h 5m"},
	}
	for _, tt := range tests {
		got := formatWorkDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatWorkDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestPrintTimeText_noCommits(t *testing.T) {
	var buf bytes.Buffer
	printTimeText(&buf, nil, prTimeMeta{Number: 1, Title: "empty PR"}, time.UTC)
	out := buf.String()
	if out == "" {
		t.Error("expected non-empty output even with no commits")
	}
}

func TestPrintTimeText_withDays(t *testing.T) {
	days := []worktime.DaySummary{
		{
			Date: "2026-06-10",
			Sessions: []worktime.Session{
				{
					Start:    time.Date(2026, 6, 10, 9, 30, 0, 0, time.UTC),
					End:      time.Date(2026, 6, 10, 10, 0, 0, 0, time.UTC),
					Duration: 30 * time.Minute,
				},
			},
			Total: 30 * time.Minute,
		},
	}
	var buf bytes.Buffer
	printTimeText(&buf, days, prTimeMeta{Number: 42, Title: "Test PR"}, time.UTC)
	out := buf.String()

	for _, want := range []string{"#42", "Test PR", "2026-06-10", "09:30", "10:00", "0h 30m"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
}
