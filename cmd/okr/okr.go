// Package okr implements the `gh wheel okr` command group, which computes
// GitHub activity metrics for OKR key-result tracking. It is designed to feed
// okr-hub's okr-metrics-sync skill: `gh wheel okr metrics` emits the same metric
// keys as okr-hub's okr_github_metrics.py, aggregated across repositories.
package okr

import "github.com/spf13/cobra"

// NewCmd returns the `gh wheel okr` parent command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "okr",
		Short: "Compute GitHub activity metrics for OKR key results",
		Long: `Compute GitHub activity metrics for OKR key-result tracking.

Designed to integrate with okr-hub: gh wheel okr metrics emits the metric keys
that okr-hub's okr-metrics-sync skill consumes, aggregated across repositories.`,
	}
	cmd.AddCommand(newMetricsCmd())
	return cmd
}
