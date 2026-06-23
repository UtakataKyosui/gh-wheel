package task

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/UtakataKyosui/gh-wheel/internal/cliexit"
	"github.com/UtakataKyosui/gh-wheel/internal/ghclient"
	"github.com/UtakataKyosui/gh-wheel/internal/jsonout"
	"github.com/UtakataKyosui/gh-wheel/internal/notify"
	"github.com/UtakataKyosui/gh-wheel/internal/repo"
	"github.com/UtakataKyosui/gh-wheel/internal/schedule"
)

// ─── JSON envelopes ─────────────────────────────────────────────────────────

type daemonView struct {
	Running bool `json:"running"`
	PID     int  `json:"pid"`
}

type scheduleEntryView struct {
	Repo          string     `json:"repo"`
	Interval      string     `json:"interval"`
	State         string     `json:"state"`
	AuthorOnly    bool       `json:"author_only"`
	ReviewOnly    bool       `json:"review_only"`
	IncludeDrafts bool       `json:"include_drafts"`
	WithReviews   bool       `json:"with_reviews"`
	Notify        bool       `json:"notify"`
	GitDir        string     `json:"git_dir"`
	LastRun       *time.Time `json:"last_run"`
	NextRun       *time.Time `json:"next_run"`
	RunCount      int        `json:"run_count"`
	LastError     string     `json:"last_error,omitempty"`
}

type scheduleResult struct {
	SchemaVersion string              `json:"schema_version"`
	Kind          string              `json:"kind"`
	Entries       []scheduleEntryView `json:"entries"`
	Daemon        daemonView          `json:"daemon"`
	TotalRuns     int                 `json:"total_runs,omitempty"`
}

func toView(e schedule.Entry) scheduleEntryView {
	v := scheduleEntryView{
		Repo:          e.Repo,
		Interval:      e.Interval,
		State:         e.State,
		AuthorOnly:    e.AuthorOnly,
		ReviewOnly:    e.ReviewOnly,
		IncludeDrafts: e.IncludeDrafts,
		WithReviews:   e.WithReviews,
		Notify:        e.Notify,
		GitDir:        e.GitDir,
		LastRun:       e.LastRun,
		RunCount:      e.RunCount,
		LastError:     e.LastError,
	}
	if e.LastRun != nil {
		n := e.NextRun()
		v.NextRun = &n
	}
	return v
}

func viewEntries(entries []schedule.Entry) []scheduleEntryView {
	out := make([]scheduleEntryView, 0, len(entries))
	for _, e := range entries {
		out = append(out, toView(e))
	}
	return out
}

// ─── command tree ───────────────────────────────────────────────────────────

func newScheduleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Periodically snapshot your tasks via a self-contained daemon",
		Long: `Run a background daemon that periodically snapshots your PR/Issue list to
<repo>/.git/my-tasks/current.json — no cron or launchd required.

Register the current repository with 'schedule add' (which starts the daemon if
needed), then 'list'/'status' to inspect it and 'stop' to halt it. With --notify,
each snapshot also raises a desktop notification summarising pending work.`,
	}
	cmd.AddCommand(
		newScheduleAddCmd(),
		newScheduleListCmd(),
		newScheduleRemoveCmd(),
		newScheduleStartCmd(),
		newScheduleStopCmd(),
		newScheduleStatusCmd(),
		newScheduleRunCmd(),
	)
	return cmd
}

// ioFlags reads the persistent --json / --jq / --dry-run flags. It tolerates a
// command invoked outside the root (e.g. in unit tests): missing flags default
// to their zero values rather than erroring.
func ioFlags(cmd *cobra.Command) (jsonOut bool, jqExpr string, dryRun bool) {
	jsonOut, _ = cmd.Flags().GetBool("json")
	jqExpr, _ = cmd.Flags().GetString("jq")
	dryRun, _ = cmd.Root().PersistentFlags().GetBool("dry-run")
	return jsonOut, jqExpr, dryRun
}

func newScheduleAddCmd() *cobra.Command {
	var (
		interval string
		opts     fetchOpts
		doNotify bool
		noStart  bool
	)
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Register the current repository with the snapshot daemon",
		Long: `Register the current repository for periodic snapshots and (unless --no-start)
start the daemon. Re-running 'add' for the same repository updates its settings
while preserving its run statistics.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			jsonOut, jqExpr, dryRun := ioFlags(cmd)
			flagRepo, _ := cmd.Flags().GetString("repo")

			if opts.State != "open" && opts.State != "closed" && opts.State != "all" {
				return cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs,
					fmt.Errorf("invalid state %q: must be one of open, closed, all", opts.State))
			}
			if opts.AuthorOnly && opts.ReviewOnly {
				return cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs,
					fmt.Errorf("--author-only and --review-only are mutually exclusive"))
			}
			if _, err := schedule.ValidateInterval(interval); err != nil {
				return cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs, err)
			}

			r, err := repo.Resolve(flagRepo)
			if err != nil {
				return err
			}
			gitDir, err := resolveGitDir()
			if err != nil {
				return err
			}

			entry := schedule.Entry{
				Repo:          r.Owner + "/" + r.Name,
				GitDir:        gitDir,
				Interval:      interval,
				State:         opts.State,
				AuthorOnly:    opts.AuthorOnly,
				ReviewOnly:    opts.ReviewOnly,
				IncludeDrafts: opts.IncludeDrafts,
				WithReviews:   opts.WithReviews,
				Notify:        doNotify,
				AddedAt:       time.Now().UTC(),
			}

			if dryRun {
				dv, derr := currentDaemonView()
				if derr != nil {
					return derr
				}
				res := scheduleResult{SchemaVersion: "v1", Kind: "schedule_add_preview", Entries: []scheduleEntryView{toView(entry)}, Daemon: dv}
				return outputSchedule(res, jsonOut, jqExpr,
					fmt.Sprintf("[dry-run] would watch %s every %s", entry.Repo, interval))
			}

			cfg, err := schedule.Load()
			if err != nil {
				return cliexit.NewGeneral(err)
			}
			cfg.Upsert(entry)
			if err := cfg.Save(); err != nil {
				return cliexit.NewGeneral(err)
			}

			var dv daemonView
			if noStart {
				dv, err = currentDaemonView()
			} else {
				dv, err = startDaemon()
			}
			if err != nil {
				return err
			}

			if doNotify && !notify.Available() && !jsonOut {
				fmt.Fprintln(os.Stderr, "note: desktop notifications are unavailable on this platform; snapshots will still run")
			}

			human := fmt.Sprintf("Watching %s every %s (daemon pid %d).", entry.Repo, interval, dv.PID)
			if !dv.Running {
				human = fmt.Sprintf("Watching %s every %s (daemon not running; run 'gh wheel task schedule start').", entry.Repo, interval)
			}
			res := scheduleResult{SchemaVersion: "v1", Kind: "schedule_add_result", Entries: viewEntries(cfg.Entries), Daemon: dv}
			return outputSchedule(res, jsonOut, jqExpr, human)
		},
	}
	cmd.Flags().StringVar(&interval, "interval", "5m", "Snapshot interval as a Go duration (e.g. 5m, 30s, 1h; minimum 1m)")
	cmd.Flags().StringVarP(&opts.State, "state", "s", "open", "Filter by state: open, closed, all")
	cmd.Flags().BoolVarP(&opts.AuthorOnly, "author-only", "a", false, "Snapshot only PRs you authored")
	cmd.Flags().BoolVarP(&opts.ReviewOnly, "review-only", "r", false, "Snapshot only PRs where review is requested from you")
	cmd.Flags().BoolVarP(&opts.IncludeDrafts, "include-drafts", "d", true, "Include draft PRs")
	cmd.Flags().BoolVar(&opts.WithReviews, "with-reviews", false, "Fetch review status for each PR (slower)")
	cmd.Flags().BoolVar(&doNotify, "notify", false, "Show a desktop notification after each snapshot")
	cmd.Flags().BoolVar(&noStart, "no-start", false, "Do not start the daemon automatically")
	return cmd
}

func newScheduleListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List watched repositories and the daemon status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			jsonOut, jqExpr, _ := ioFlags(cmd)
			cfg, err := schedule.Load()
			if err != nil {
				return cliexit.NewGeneral(err)
			}
			dv, err := currentDaemonView()
			if err != nil {
				return err
			}
			res := scheduleResult{SchemaVersion: "v1", Kind: "schedule_list_result", Entries: viewEntries(cfg.Entries), Daemon: dv}
			if jsonOut {
				return jsonout.Print(res, jqExpr)
			}
			printScheduleList(os.Stdout, res)
			return nil
		},
	}
}

func newScheduleRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove",
		Short: "Unregister the current repository from the snapshot daemon",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			jsonOut, jqExpr, dryRun := ioFlags(cmd)
			flagRepo, _ := cmd.Flags().GetString("repo")

			r, err := repo.Resolve(flagRepo)
			if err != nil {
				return err
			}
			repoName := r.Owner + "/" + r.Name

			cfg, err := schedule.Load()
			if err != nil {
				return cliexit.NewGeneral(err)
			}
			if _, i := cfg.Find(repoName); i < 0 {
				return cliexit.NewNotFound(cliexit.ErrCodeNotFound,
					fmt.Errorf("%s is not registered with the snapshot daemon", repoName))
			}

			if dryRun {
				dv, derr := currentDaemonView()
				if derr != nil {
					return derr
				}
				res := scheduleResult{SchemaVersion: "v1", Kind: "schedule_remove_preview", Entries: viewEntries(cfg.Entries), Daemon: dv}
				return outputSchedule(res, jsonOut, jqExpr, fmt.Sprintf("[dry-run] would stop watching %s", repoName))
			}

			cfg.Remove(repoName)
			if err := cfg.Save(); err != nil {
				return cliexit.NewGeneral(err)
			}

			dv, err := currentDaemonView()
			if err != nil {
				return err
			}
			human := fmt.Sprintf("Stopped watching %s.", repoName)
			if len(cfg.Entries) == 0 {
				stopped, pid, serr := schedule.Stop()
				if serr != nil {
					return cliexit.NewGeneral(fmt.Errorf("stop daemon after removing last repository: %w", serr))
				}
				if stopped {
					dv = daemonView{Running: false, PID: 0}
					human += fmt.Sprintf(" No repositories left; daemon stopped (was pid %d).", pid)
				}
			}
			res := scheduleResult{SchemaVersion: "v1", Kind: "schedule_remove_result", Entries: viewEntries(cfg.Entries), Daemon: dv}
			return outputSchedule(res, jsonOut, jqExpr, human)
		},
	}
}

func newScheduleStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the snapshot daemon",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			jsonOut, jqExpr, _ := ioFlags(cmd)
			exe, err := os.Executable()
			if err != nil {
				return cliexit.NewGeneral(fmt.Errorf("resolve executable: %w", err))
			}
			pid, started, err := schedule.Start(exe)
			if err != nil {
				return cliexit.NewGeneral(err)
			}
			human := fmt.Sprintf("Snapshot daemon started (pid %d).", pid)
			if !started {
				human = fmt.Sprintf("Snapshot daemon already running (pid %d).", pid)
			}
			res := scheduleResult{SchemaVersion: "v1", Kind: "schedule_start_result", Daemon: daemonView{Running: true, PID: pid}}
			return outputSchedule(res, jsonOut, jqExpr, human)
		},
	}
}

func newScheduleStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the snapshot daemon (SIGTERM)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			jsonOut, jqExpr, _ := ioFlags(cmd)
			stopped, pid, err := schedule.Stop()
			if err != nil {
				return cliexit.NewGeneral(err)
			}
			human := "Snapshot daemon was not running."
			if stopped {
				human = fmt.Sprintf("Snapshot daemon stopped (was pid %d).", pid)
			}
			res := scheduleResult{SchemaVersion: "v1", Kind: "schedule_stop_result", Daemon: daemonView{Running: false, PID: 0}}
			return outputSchedule(res, jsonOut, jqExpr, human)
		},
	}
}

func newScheduleStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the daemon pid and per-repo run counts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			jsonOut, jqExpr, _ := ioFlags(cmd)
			cfg, err := schedule.Load()
			if err != nil {
				return cliexit.NewGeneral(err)
			}
			dv, err := currentDaemonView()
			if err != nil {
				return err
			}
			total := 0
			for _, e := range cfg.Entries {
				total += e.RunCount
			}
			res := scheduleResult{SchemaVersion: "v1", Kind: "schedule_status_result", Entries: viewEntries(cfg.Entries), Daemon: dv, TotalRuns: total}
			if jsonOut {
				return jsonout.Print(res, jqExpr)
			}
			printScheduleList(os.Stdout, res)
			fmt.Fprintf(os.Stdout, "total snapshots taken: %d\n", total)
			return nil
		},
	}
}

// newScheduleRunCmd is the hidden daemon entry point re-exec'd by Start.
func newScheduleRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "__run",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return schedule.Run(cmd.Context(), snapshotEntry, notify.Notify)
		},
	}
}

// ─── helpers ────────────────────────────────────────────────────────────────

// summarize counts PRs awaiting your review and PRs you authored from a snapshot.
func summarize(repoName string, r *TaskResult) schedule.Summary {
	s := schedule.Summary{Repo: repoName}
	if r == nil {
		return s
	}
	for _, pr := range r.PRs {
		for _, cat := range pr.Categories {
			switch cat {
			case "review-requested":
				s.ReviewRequested++
			case "author":
				s.Authored++
			}
		}
	}
	return s
}

// snapshotEntry is the SnapshotFunc the daemon calls for each due repository.
// It honours ctx by aborting before issuing any GitHub request once the daemon
// is shutting down, so SIGTERM is not held up by a fetch about to start.
func snapshotEntry(ctx context.Context, e schedule.Entry) ([]byte, schedule.Summary, error) {
	if err := ctx.Err(); err != nil {
		return nil, schedule.Summary{}, err
	}
	owner, name, _ := strings.Cut(e.Repo, "/")
	c, err := ghclient.NewForRepo(owner, name)
	if err != nil {
		return nil, schedule.Summary{}, err
	}
	login, err := c.CurrentUser()
	if err != nil {
		return nil, schedule.Summary{}, err
	}
	result, err := fetch(c, login, fetchOpts{
		State:         e.State,
		AuthorOnly:    e.AuthorOnly,
		ReviewOnly:    e.ReviewOnly,
		IncludeDrafts: e.IncludeDrafts,
		WithReviews:   e.WithReviews,
	})
	if err != nil {
		return nil, schedule.Summary{}, err
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, schedule.Summary{}, cliexit.NewGeneral(fmt.Errorf("marshal snapshot: %w", err))
	}
	return append(data, '\n'), summarize(e.Repo, result), nil
}

// resolveGitDir returns the absolute .git directory of the current repository,
// where the snapshot for this repo will be written.
func resolveGitDir() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--absolute-git-dir").Output()
	if err != nil {
		return "", cliexit.NewUsage(cliexit.ErrCodeUsageNoRepo,
			fmt.Errorf("could not locate the .git directory; run 'schedule add' inside a git repository"))
	}
	return strings.TrimSpace(string(out)), nil
}

func currentDaemonView() (daemonView, error) {
	running, pid, err := schedule.Running()
	if err != nil {
		return daemonView{}, cliexit.NewGeneral(err)
	}
	return daemonView{Running: running, PID: pid}, nil
}

func startDaemon() (daemonView, error) {
	exe, err := os.Executable()
	if err != nil {
		return daemonView{}, cliexit.NewGeneral(fmt.Errorf("resolve executable: %w", err))
	}
	pid, _, err := schedule.Start(exe)
	if err != nil {
		return daemonView{}, cliexit.NewGeneral(err)
	}
	return daemonView{Running: true, PID: pid}, nil
}

func outputSchedule(res scheduleResult, jsonOut bool, jqExpr, human string) error {
	if res.Entries == nil {
		res.Entries = []scheduleEntryView{}
	}
	if jsonOut {
		return jsonout.Print(res, jqExpr)
	}
	if human != "" {
		fmt.Fprintln(os.Stdout, human)
	}
	return nil
}

func printScheduleList(w io.Writer, res scheduleResult) {
	if res.Daemon.Running {
		fmt.Fprintf(w, "daemon: running (pid %d)\n", res.Daemon.PID)
	} else {
		fmt.Fprintln(w, "daemon: not running")
	}
	if len(res.Entries) == 0 {
		fmt.Fprintln(w, "no repositories registered")
		return
	}
	for _, e := range res.Entries {
		last := "never"
		if e.LastRun != nil {
			last = e.LastRun.Format(time.RFC3339)
		}
		notifyMark := ""
		if e.Notify {
			notifyMark = " 🔔"
		}
		fmt.Fprintf(w, "  %s  every %s  runs=%d  last=%s%s\n", e.Repo, e.Interval, e.RunCount, last, notifyMark)
		if e.LastError != "" {
			fmt.Fprintf(w, "      last error: %s\n", e.LastError)
		}
	}
}
