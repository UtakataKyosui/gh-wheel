package task

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/UtakataKyosui/gh-wheel/internal/cliexit"
	"github.com/UtakataKyosui/gh-wheel/internal/ghclient"
	"github.com/UtakataKyosui/gh-wheel/internal/jsonout"
	"github.com/UtakataKyosui/gh-wheel/internal/worktime"
)

type prTimeCommit struct {
	Commit struct {
		Committer struct {
			Date string `json:"date"`
		} `json:"committer"`
	} `json:"commit"`
}

type prTimeMeta struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
}

type timeResult struct {
	SchemaVersion string           `json:"schema_version"`
	Kind          string           `json:"kind"`
	PR            int              `json:"pr"`
	Title         string           `json:"title"`
	Params        timeResultParams `json:"params"`
	Days          []timeDaySummary `json:"days"`
	TotalMinutes  int              `json:"total_minutes"`
}

type timeResultParams struct {
	SessionGapMinutes int    `json:"session_gap_minutes"`
	LeadTimeMinutes   int    `json:"lead_time_minutes"`
	MinSessionMinutes int    `json:"min_session_minutes"`
	Timezone          string `json:"timezone"`
}

type timeDaySummary struct {
	Date         string        `json:"date"`
	Sessions     []timeSession `json:"sessions"`
	TotalMinutes int           `json:"total_minutes"`
}

type timeSession struct {
	Start           string `json:"start"`
	End             string `json:"end"`
	DurationMinutes int    `json:"duration_minutes"`
}

// newTimeCmd returns the `gh wheel task time <PR>` subcommand.
func newTimeCmd() *cobra.Command {
	var (
		sessionGap int
		leadTime   int
		minSession int
		tz         string
	)

	cmd := &cobra.Command{
		Use:   "time <PR>",
		Short: "Show estimated work time breakdown for a PR by session",
		Long: `Analyze PR commits and estimate actual work time using a session-based algorithm.

A "session" is a continuous work block. Gaps longer than --session-gap minutes
between commits start a new session. Each session's start is estimated by
subtracting --lead-time minutes from the first commit (crediting work done
before committing). Single-commit sessions are guaranteed at least --min-session
minutes.

Example:
  gh wheel task time 42
  gh wheel task time 42 --tz Asia/Tokyo --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prNum, err := strconv.Atoi(args[0])
			if err != nil || prNum <= 0 {
				return cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs,
					fmt.Errorf("invalid PR number %q: must be a positive integer", args[0]))
			}

			loc, err := time.LoadLocation(tz)
			if err != nil {
				return cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs,
					fmt.Errorf("invalid timezone %q: %w", tz, err))
			}

			flagRepo, _ := cmd.Flags().GetString("repo")
			jsonMode, _ := cmd.Flags().GetBool("json")
			jqExpr, _ := cmd.Flags().GetString("jq")

			c, err := ghclient.New(flagRepo)
			if err != nil {
				return err
			}

			var meta prTimeMeta
			if err := c.RepoGet(fmt.Sprintf("pulls/%d", prNum), &meta); err != nil {
				return err
			}

			var rawCommits []prTimeCommit
			if err := c.RepoGet(fmt.Sprintf("pulls/%d/commits?per_page=100", prNum), &rawCommits); err != nil {
				return err
			}

			times := make([]time.Time, 0, len(rawCommits))
			for _, rc := range rawCommits {
				t, err := time.Parse(time.RFC3339, rc.Commit.Committer.Date)
				if err != nil {
					continue
				}
				times = append(times, t)
			}

			p := worktime.Params{
				SessionGap: time.Duration(sessionGap) * time.Minute,
				LeadTime:   time.Duration(leadTime) * time.Minute,
				MinSession: time.Duration(minSession) * time.Minute,
				Location:   loc,
			}
			days := worktime.Calculate(times, p)

			if jsonMode {
				return printTimeJSON(days, meta, p, tz, jqExpr)
			}
			printTimeText(os.Stdout, days, meta, loc)
			return nil
		},
	}

	cmd.Flags().IntVar(&sessionGap, "session-gap", worktime.DefaultSessionGapMinutes,
		"Gap in minutes between commits that starts a new session")
	cmd.Flags().IntVar(&leadTime, "lead-time", worktime.DefaultLeadTimeMinutes,
		"Minutes added before the first commit of each session")
	cmd.Flags().IntVar(&minSession, "min-session", worktime.DefaultMinSessionMinutes,
		"Minimum session duration in minutes")
	cmd.Flags().StringVar(&tz, "tz", "Local",
		`Timezone for date grouping (e.g. "Asia/Tokyo")`)

	return cmd
}

func printTimeJSON(days []worktime.DaySummary, meta prTimeMeta, p worktime.Params, tz, jqExpr string) error {
	var total int
	daySummaries := make([]timeDaySummary, 0, len(days))
	for _, d := range days {
		sessions := make([]timeSession, 0, len(d.Sessions))
		for _, s := range d.Sessions {
			sessions = append(sessions, timeSession{
				Start:           s.Start.In(p.Location).Format("15:04"),
				End:             s.End.In(p.Location).Format("15:04"),
				DurationMinutes: int(s.Duration.Minutes()),
			})
		}
		daySummaries = append(daySummaries, timeDaySummary{
			Date:         d.Date,
			Sessions:     sessions,
			TotalMinutes: int(d.Total.Minutes()),
		})
		total += int(d.Total.Minutes())
	}

	result := timeResult{
		SchemaVersion: "v1",
		Kind:          "task_time_result",
		PR:            meta.Number,
		Title:         meta.Title,
		Params: timeResultParams{
			SessionGapMinutes: int(p.SessionGap.Minutes()),
			LeadTimeMinutes:   int(p.LeadTime.Minutes()),
			MinSessionMinutes: int(p.MinSession.Minutes()),
			Timezone:          tz,
		},
		Days:         daySummaries,
		TotalMinutes: total,
	}
	return jsonout.Print(result, jqExpr)
}

func printTimeText(w io.Writer, days []worktime.DaySummary, meta prTimeMeta, loc *time.Location) {
	fmt.Fprintf(w, "#%d %s\n\n", meta.Number, meta.Title)
	if len(days) == 0 {
		fmt.Fprintln(w, "No commits found.")
		return
	}
	var total time.Duration
	for _, d := range days {
		fmt.Fprintf(w, "%s\n", d.Date)
		for _, s := range d.Sessions {
			fmt.Fprintf(w, "  %s - %s  (%s)\n",
				s.Start.In(loc).Format("15:04"),
				s.End.In(loc).Format("15:04"),
				formatWorkDuration(s.Duration))
		}
		fmt.Fprintf(w, "  subtotal: %s\n\n", formatWorkDuration(d.Total))
		total += d.Total
	}
	fmt.Fprintf(w, "Total: %s\n", formatWorkDuration(total))
}

func formatWorkDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", h, m)
}
