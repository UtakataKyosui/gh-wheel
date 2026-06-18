package okr

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewMetricsCmd(t *testing.T) {
	cmd := newMetricsCmd()
	if !strings.HasPrefix(cmd.Use, "metrics") {
		t.Errorf("Use = %q, want prefix %q", cmd.Use, "metrics")
	}
	if cmd.Short == "" || cmd.Long == "" {
		t.Error("Short and Long must be non-empty")
	}
	for _, name := range []string{"since", "until", "krs"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not defined", name)
		}
	}
	// since and until are required.
	for _, name := range []string{"since", "until"} {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Fatalf("flag --%s missing", name)
		}
		if f.Annotations["cobra_annotation_bash_completion_one_required_flag"] == nil {
			t.Errorf("flag --%s should be required", name)
		}
	}
}

func TestValidateDate(t *testing.T) {
	tests := []struct {
		in      string
		wantErr bool
	}{
		{"2026-06-18", false},
		{"", true},
		{"2026-6-18", true},
		{"18-06-2026", true},
		{"not-a-date", true},
		{"2026-13-01", true},
	}
	for _, tt := range tests {
		err := validateDate(tt.in)
		if tt.wantErr && err == nil {
			t.Errorf("validateDate(%q) = nil, want error", tt.in)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("validateDate(%q) = %v, want nil", tt.in, err)
		}
	}
}

func TestWriteText(t *testing.T) {
	cyc := 18.4
	r := metricsResult{
		Scope: "all-repos",
		Since: "2026-01-01",
		Until: "2026-06-30",
		Metrics: metrics{
			AuthoredPRsTotal:       12,
			MergedPRs:              9,
			AvgCycleTimeHours:      &cyc,
			ReviewCommentsReceived: 23,
			AvgReviewCommentsPerPR: 1.92,
			ReviewedPRs:            7,
			IssuesCreated:          5,
			IssuesClosed:           4,
		},
		KRMetrics: map[string]krMetric{
			"KR1": {Source: "github:pr_count", Value: 12, KRTitle: "出力"},
		},
	}

	var buf bytes.Buffer
	if err := writeText(&buf, r); err != nil {
		t.Fatalf("writeText: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"all-repos", "2026-01-01", "12", "18.4", "1.92", "KR1"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
}

func TestWriteText_NilCycleTime(t *testing.T) {
	r := metricsResult{Scope: "all-repos", Metrics: metrics{AvgCycleTimeHours: nil}}
	var buf bytes.Buffer
	if err := writeText(&buf, r); err != nil {
		t.Fatalf("writeText: %v", err)
	}
	if !strings.Contains(buf.String(), "n/a") {
		t.Errorf("nil cycle time should render as n/a:\n%s", buf.String())
	}
}
