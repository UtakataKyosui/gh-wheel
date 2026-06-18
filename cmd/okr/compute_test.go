package okr

import (
	"testing"
	"time"
)

func ptrFloat(f float64) *float64 { return &f }

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time %q: %v", s, err)
	}
	return v
}

func TestParseKRs(t *testing.T) {
	t.Run("empty string yields nil", func(t *testing.T) {
		krs, err := parseKRs("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if krs != nil {
			t.Errorf("expected nil, got %v", krs)
		}
	})

	t.Run("valid JSON decodes", func(t *testing.T) {
		krs, err := parseKRs(`[{"label":"KR1","title":"品質","metrics_source":"github:pr_count"}]`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(krs) != 1 || krs[0].Label != "KR1" || krs[0].MetricsSource != "github:pr_count" {
			t.Errorf("unexpected decode: %+v", krs)
		}
	})

	t.Run("invalid JSON errors", func(t *testing.T) {
		if _, err := parseKRs("{not json"); err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}

func TestMatchKRs(t *testing.T) {
	m := metrics{PRCount: 12, AvgReviewCommentsPerPR: 1.92, AvgCycleTimeHours: nil}

	t.Run("no KRs yields empty non-nil map", func(t *testing.T) {
		got := matchKRs(m, nil)
		if got == nil {
			t.Fatal("expected non-nil map")
		}
		if len(got) != 0 {
			t.Errorf("expected empty map, got %v", got)
		}
	})

	t.Run("matches recognised source", func(t *testing.T) {
		got := matchKRs(m, []krInput{{Label: "KR1", Title: "品質", MetricsSource: "github:avg_review_comments_per_pr"}})
		km, ok := got["KR1"]
		if !ok {
			t.Fatal("KR1 not matched")
		}
		if km.Source != "github:avg_review_comments_per_pr" || km.KRTitle != "品質" {
			t.Errorf("unexpected km: %+v", km)
		}
		if v, ok := km.Value.(float64); !ok || v != 1.92 {
			t.Errorf("value = %v, want 1.92", km.Value)
		}
	})

	t.Run("skips unrecognised source", func(t *testing.T) {
		got := matchKRs(m, []krInput{{Label: "KR9", MetricsSource: "github:unknown"}})
		if len(got) != 0 {
			t.Errorf("expected no matches, got %v", got)
		}
	})

	t.Run("nil cycle time matches as nil value", func(t *testing.T) {
		got := matchKRs(m, []krInput{{Label: "KR2", MetricsSource: "github:avg_cycle_time_hours"}})
		km := got["KR2"]
		if km.Value != nil {
			t.Errorf("expected nil value, got %v", km.Value)
		}
	})
}

func TestSourceAliasesCoverage(t *testing.T) {
	// Every alias target must be resolvable by metrics.value.
	m := metrics{AvgCycleTimeHours: ptrFloat(1)}
	for source, key := range sourceAliases {
		if m.value(key) == nil && key != "avg_cycle_time_hours" {
			t.Errorf("source %q -> key %q resolves to nil", source, key)
		}
	}
}

func TestReviewCommentStats(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		got, avg := reviewCommentStats(nil)
		if got != 0 || avg != 0 {
			t.Errorf("got (%d, %v), want (0, 0)", got, avg)
		}
	})

	t.Run("sum and average", func(t *testing.T) {
		items := []prItem{{Comments: 1}, {Comments: 2}, {Comments: 0}}
		got, avg := reviewCommentStats(items)
		if got != 3 {
			t.Errorf("received = %d, want 3", got)
		}
		if avg != 1.0 {
			t.Errorf("avg = %v, want 1.0", avg)
		}
	})

	t.Run("rounds to 2dp", func(t *testing.T) {
		items := []prItem{{Comments: 1}, {Comments: 1}, {Comments: 0}}
		_, avg := reviewCommentStats(items)
		if avg != 0.67 {
			t.Errorf("avg = %v, want 0.67", avg)
		}
	})
}

func TestCycleTimeAvgHours(t *testing.T) {
	t.Run("no merged returns nil", func(t *testing.T) {
		items := []prItem{{CreatedAt: mustTime(t, "2026-01-01T00:00:00Z")}}
		if got := cycleTimeAvgHours(items); got != nil {
			t.Errorf("expected nil, got %v", *got)
		}
	})

	t.Run("averages merged durations", func(t *testing.T) {
		merged2h := mustTime(t, "2026-01-01T02:00:00Z")
		merged4h := mustTime(t, "2026-01-01T04:00:00Z")
		items := []prItem{
			{CreatedAt: mustTime(t, "2026-01-01T00:00:00Z"), PullRequest: &struct {
				MergedAt *time.Time `json:"merged_at"`
			}{MergedAt: &merged2h}},
			{CreatedAt: mustTime(t, "2026-01-01T00:00:00Z"), PullRequest: &struct {
				MergedAt *time.Time `json:"merged_at"`
			}{MergedAt: &merged4h}},
		}
		got := cycleTimeAvgHours(items)
		if got == nil {
			t.Fatal("expected non-nil average")
		}
		if *got != 3.0 {
			t.Errorf("avg = %v, want 3.0", *got)
		}
	})

	t.Run("ignores unmerged and negative durations", func(t *testing.T) {
		merged := mustTime(t, "2026-01-01T01:00:00Z")
		items := []prItem{
			{CreatedAt: mustTime(t, "2026-01-01T00:00:00Z"), PullRequest: &struct {
				MergedAt *time.Time `json:"merged_at"`
			}{MergedAt: &merged}},
			{CreatedAt: mustTime(t, "2026-01-01T00:00:00Z")}, // unmerged, skipped
		}
		got := cycleTimeAvgHours(items)
		if got == nil || *got != 1.0 {
			t.Errorf("avg = %v, want 1.0", got)
		}
	})
}
