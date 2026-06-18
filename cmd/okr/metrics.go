package okr

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/UtakataKyosui/gh-wheel/internal/cliexit"
	"github.com/UtakataKyosui/gh-wheel/internal/ghclient"
	"github.com/UtakataKyosui/gh-wheel/internal/jsonout"
)

// metricsResult is the top-level JSON output for `gh wheel okr metrics`.
// It is a superset of okr-hub's okr_github_metrics.py output (metrics +
// kr_metrics) wrapped in gh-wheel's schema_version/kind envelope.
type metricsResult struct {
	SchemaVersion string              `json:"schema_version"`
	Kind          string              `json:"kind"`
	Available     bool                `json:"available"`
	Scope         string              `json:"scope"`
	Repo          string              `json:"repo"`
	Since         string              `json:"since"`
	Until         string              `json:"until"`
	Metrics       metrics             `json:"metrics"`
	KRMetrics     map[string]krMetric `json:"kr_metrics"`
}

func newMetricsCmd() *cobra.Command {
	var (
		flagSince string
		flagUntil string
		flagKRs   string
	)

	cmd := &cobra.Command{
		Use:   "metrics --since <YYYY-MM-DD> --until <YYYY-MM-DD>",
		Short: "Compute period GitHub metrics (cross-repo) for okr-hub KR sync",
		Long: `Compute GitHub activity metrics over a date range.

By default the search is cross-repo (author:@me / reviewed-by:@me across every
repository you can see), which suits personal OKRs. Pass -R owner/repo to scope
the metrics to a single repository.

The output uses the same metric keys as okr-hub's okr-metrics-sync skill, so it
can replace plugins/okr-progress/scripts/okr_github_metrics.py. Pass --krs with
the JSON produced by okr_parse.py to map each metric onto a key result.

Authentication failures surface as exit code 4 with an error envelope (not an
"available": false body); a successful run always reports "available": true.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			flagRepo, _ := cmd.Flags().GetString("repo")
			jsonOut, _ := cmd.Flags().GetBool("json")
			jqExpr, _ := cmd.Flags().GetString("jq")

			if err := validateDate(flagSince); err != nil {
				return cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs, fmt.Errorf("--since: %w", err))
			}
			if err := validateDate(flagUntil); err != nil {
				return cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs, fmt.Errorf("--until: %w", err))
			}
			if flagSince > flagUntil {
				return cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs,
					fmt.Errorf("--since (%s) must not be after --until (%s)", flagSince, flagUntil))
			}

			krs, err := parseKRs(flagKRs)
			if err != nil {
				return cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs, fmt.Errorf("--krs: %w", err))
			}

			c, scope, repo, err := newClient(flagRepo)
			if err != nil {
				return err
			}

			m, err := gatherMetrics(c, repo, flagSince, flagUntil)
			if err != nil {
				return err
			}

			result := metricsResult{
				SchemaVersion: "v1",
				Kind:          "okr_metrics",
				Available:     true,
				Scope:         scope,
				Repo:          repo,
				Since:         flagSince,
				Until:         flagUntil,
				Metrics:       m,
				KRMetrics:     matchKRs(m, krs),
			}

			if jsonOut {
				return jsonout.Print(result, jqExpr)
			}
			return writeText(os.Stdout, result)
		},
	}

	cmd.Flags().StringVar(&flagSince, "since", "", "Start date (YYYY-MM-DD, inclusive) [required]")
	cmd.Flags().StringVar(&flagUntil, "until", "", "End date (YYYY-MM-DD, inclusive) [required]")
	cmd.Flags().StringVar(&flagKRs, "krs", "", `Key results to match, as JSON: [{"label","title","metrics_source"}]`)
	_ = cmd.MarkFlagRequired("since")
	_ = cmd.MarkFlagRequired("until")
	return cmd
}

// newClient returns a GitHub client plus the scope label and repo string.
// When flagRepo is set the client is repo-scoped; otherwise it is user-scoped
// (cross-repo) and does not require the cwd to be a git repository.
func newClient(flagRepo string) (c *ghclient.Client, scope, repo string, err error) {
	if strings.TrimSpace(flagRepo) != "" {
		c, err = ghclient.New(flagRepo)
		if err != nil {
			return nil, "", "", err
		}
		repo = fmt.Sprintf("%s/%s", c.Owner(), c.Name())
		return c, repo, repo, nil
	}
	c, err = ghclient.NewUserScoped()
	if err != nil {
		return nil, "", "", err
	}
	return c, "all-repos", "", nil
}

// validateDate checks that s is a non-empty YYYY-MM-DD date.
func validateDate(s string) error {
	if s == "" {
		return fmt.Errorf("date is required")
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return fmt.Errorf("invalid date %q: expected YYYY-MM-DD", s)
	}
	return nil
}

// writeText renders a human-readable summary (used when --json is absent).
func writeText(w io.Writer, r metricsResult) error {
	var b strings.Builder
	fmt.Fprintf(&b, "OKR GitHub metrics (%s .. %s)  scope=%s\n", r.Since, r.Until, r.Scope)
	m := r.Metrics
	fmt.Fprintf(&b, "  authored PRs     : %d\n", m.AuthoredPRsTotal)
	fmt.Fprintf(&b, "  merged PRs       : %d\n", m.MergedPRs)
	cycle := "n/a"
	if m.AvgCycleTimeHours != nil {
		cycle = fmt.Sprintf("%.1f h", *m.AvgCycleTimeHours)
	}
	fmt.Fprintf(&b, "  avg cycle time   : %s\n", cycle)
	fmt.Fprintf(&b, "  review comments  : %d (avg %.2f/PR)\n", m.ReviewCommentsReceived, m.AvgReviewCommentsPerPR)
	fmt.Fprintf(&b, "  reviewed PRs     : %d\n", m.ReviewedPRs)
	fmt.Fprintf(&b, "  issues created   : %d\n", m.IssuesCreated)
	fmt.Fprintf(&b, "  issues closed    : %d\n", m.IssuesClosed)

	if len(r.KRMetrics) > 0 {
		fmt.Fprintln(&b, "  KR matches:")
		labels := make([]string, 0, len(r.KRMetrics))
		for label := range r.KRMetrics {
			labels = append(labels, label)
		}
		sort.Strings(labels)
		for _, label := range labels {
			km := r.KRMetrics[label]
			fmt.Fprintf(&b, "    %s (%s): %v  [%s]\n", label, km.Source, km.Value, km.KRTitle)
		}
	}

	_, err := fmt.Fprint(w, b.String())
	return err
}
