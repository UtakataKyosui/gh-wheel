package worktime_test

import (
	"testing"
	"time"

	"github.com/UtakataKyosui/gh-wheel/internal/worktime"
)

var jst = time.FixedZone("JST", 9*60*60)

func mustParse(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestCalculate_empty(t *testing.T) {
	result := worktime.Calculate(nil, worktime.DefaultParams())
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d days", len(result))
	}
}

func TestCalculate_singleCommit(t *testing.T) {
	p := worktime.DefaultParams()
	commits := []time.Time{mustParse("2026-06-10T10:00:00Z")}
	days := worktime.Calculate(commits, p)

	if len(days) != 1 {
		t.Fatalf("expected 1 day, got %d", len(days))
	}
	d := days[0]
	// Single commit: duration = max(LEAD_TIME, MIN_SESSION) = max(30m, 15m) = 30m
	if d.Total != 30*time.Minute {
		t.Errorf("single commit total: want 30m, got %v", d.Total)
	}
	if len(d.Sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(d.Sessions))
	}
	// Session start = commit - LEAD_TIME = 09:30
	wantStart := mustParse("2026-06-10T09:30:00Z")
	if !d.Sessions[0].Start.Equal(wantStart) {
		t.Errorf("session start: want %v, got %v", wantStart, d.Sessions[0].Start)
	}
}

func TestCalculate_twoCommitsGapExceedsThreshold(t *testing.T) {
	p := worktime.DefaultParams()
	// Gap = 45m > SESSION_GAP 30m → 2 sessions
	commits := []time.Time{
		mustParse("2026-06-10T10:00:00Z"),
		mustParse("2026-06-10T10:45:00Z"),
	}
	days := worktime.Calculate(commits, p)

	if len(days) != 1 {
		t.Fatalf("expected 1 day, got %d", len(days))
	}
	if len(days[0].Sessions) != 2 {
		t.Errorf("expected 2 sessions for 45m gap > SESSION_GAP, got %d", len(days[0].Sessions))
	}
	// Session1: start=09:30, end=10:00, duration=30m
	// Session2: start=10:15, end=10:45, duration=30m
	// Total = 60m
	if days[0].Total != 60*time.Minute {
		t.Errorf("total: want 60m, got %v", days[0].Total)
	}
}

func TestCalculate_twoCommitsNoGap(t *testing.T) {
	p := worktime.DefaultParams()
	// Gap = 20 min < SESSION_GAP 30 min → same session
	commits := []time.Time{
		mustParse("2026-06-10T10:00:00Z"),
		mustParse("2026-06-10T10:20:00Z"),
	}
	days := worktime.Calculate(commits, p)

	if len(days) != 1 {
		t.Fatalf("expected 1 day, got %d", len(days))
	}
	if len(days[0].Sessions) != 1 {
		t.Errorf("expected 1 session for 20m gap < SESSION_GAP, got %d", len(days[0].Sessions))
	}
	// start = 10:00 - 30m = 09:30, end = 10:20, duration = 50m
	want := 50 * time.Minute
	if days[0].Total != want {
		t.Errorf("total: want %v, got %v", want, days[0].Total)
	}
}

func TestCalculate_gapSplitsIntoTwoSessions(t *testing.T) {
	p := worktime.DefaultParams()
	// Gap = 3h15m >> SESSION_GAP → 2 sessions
	commits := []time.Time{
		mustParse("2026-06-10T10:00:00Z"),
		mustParse("2026-06-10T14:00:00Z"),
	}
	days := worktime.Calculate(commits, p)

	if len(days) != 1 {
		t.Fatalf("expected 1 day, got %d", len(days))
	}
	if len(days[0].Sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(days[0].Sessions))
	}
	// Session1: start=09:30, end=10:00, duration=30m
	// Session2: start=13:30, end=14:00, duration=30m
	// Total = 60m
	if days[0].Total != 60*time.Minute {
		t.Errorf("total: want 60m, got %v", days[0].Total)
	}
}

func TestCalculate_multipleCommitsInSession(t *testing.T) {
	p := worktime.DefaultParams()
	// 3 commits within 20 min gaps → 1 session
	commits := []time.Time{
		mustParse("2026-06-10T10:00:00Z"),
		mustParse("2026-06-10T10:15:00Z"),
		mustParse("2026-06-10T10:28:00Z"),
	}
	days := worktime.Calculate(commits, p)

	if len(days[0].Sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(days[0].Sessions))
	}
	// start = 10:00 - 30m = 09:30, end = 10:28, duration = 58m
	want := 58 * time.Minute
	if days[0].Total != want {
		t.Errorf("total: want %v, got %v", want, days[0].Total)
	}
}

func TestCalculate_multiDay(t *testing.T) {
	p := worktime.DefaultParams()
	commits := []time.Time{
		mustParse("2026-06-10T10:00:00Z"),
		mustParse("2026-06-11T09:00:00Z"),
	}
	days := worktime.Calculate(commits, p)

	if len(days) != 2 {
		t.Fatalf("expected 2 days, got %d", len(days))
	}
	if days[0].Date != "2026-06-10" {
		t.Errorf("day0: want 2026-06-10, got %s", days[0].Date)
	}
	if days[1].Date != "2026-06-11" {
		t.Errorf("day1: want 2026-06-11, got %s", days[1].Date)
	}
}

func TestCalculate_timezoneAffectsDateGrouping(t *testing.T) {
	// Commit at 2026-06-10T23:30:00Z = 2026-06-11T08:30:00 JST
	p := worktime.DefaultParams()
	p.Location = jst
	commits := []time.Time{mustParse("2026-06-10T23:30:00Z")}
	days := worktime.Calculate(commits, p)

	if len(days) != 1 {
		t.Fatalf("expected 1 day, got %d", len(days))
	}
	// In JST this is 2026-06-11
	if days[0].Date != "2026-06-11" {
		t.Errorf("date should be 2026-06-11 in JST, got %s", days[0].Date)
	}
}

func TestCalculate_minSessionGuard(t *testing.T) {
	// LEAD_TIME=5m, MIN_SESSION=15m → single commit gets 15m (not 5m)
	p := worktime.Params{
		SessionGap: 30 * time.Minute,
		LeadTime:   5 * time.Minute,
		MinSession: 15 * time.Minute,
		Location:   time.UTC,
	}
	commits := []time.Time{mustParse("2026-06-10T10:00:00Z")}
	days := worktime.Calculate(commits, p)

	if days[0].Total != 15*time.Minute {
		t.Errorf("min session guard: want 15m, got %v", days[0].Total)
	}
}

func TestCalculate_unsortedInput(t *testing.T) {
	p := worktime.DefaultParams()
	// Provide commits in reverse order
	commits := []time.Time{
		mustParse("2026-06-10T10:20:00Z"),
		mustParse("2026-06-10T10:00:00Z"),
	}
	days := worktime.Calculate(commits, p)

	if len(days[0].Sessions) != 1 {
		t.Errorf("unsorted input should produce 1 session after sort, got %d", len(days[0].Sessions))
	}
}
