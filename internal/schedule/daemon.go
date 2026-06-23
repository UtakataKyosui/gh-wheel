package schedule

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// maxPoll bounds how long the daemon sleeps between config re-reads, so a repo
// added (or removed) while the daemon is running is noticed promptly even when
// the next snapshot is far off.
const maxPoll = 30 * time.Second

// Summary is the per-repo result of a snapshot, used to drive notifications.
type Summary struct {
	Repo            string
	ReviewRequested int // PRs awaiting your review
	Authored        int // PRs you authored
}

// SnapshotFunc fetches the task snapshot for an entry, returning the JSON bytes
// to persist and a summary for notifications. It is injected by the caller so
// this package stays free of GitHub specifics (and is unit-testable). The
// context is cancelled when the daemon is shutting down (SIGTERM/SIGINT), so a
// long-running fetch can abort promptly instead of delaying graceful shutdown.
type SnapshotFunc func(ctx context.Context, e Entry) (data []byte, summary Summary, err error)

// Notifier shows a desktop notification. Injected for the same reasons.
type Notifier func(title, message string) error

// ─── process state ────────────────────────────────────────────────────────────

// Running reports whether the daemon is running, returning its pid if so.
// A missing or corrupt pid file, or a pid whose process is gone, all read as
// "not running" (the stale file is left for Stop/Start to clean up).
func Running() (bool, int, error) {
	p, err := PidPath()
	if err != nil {
		return false, 0, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, fmt.Errorf("read pid file: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return false, 0, nil
	}
	if !alive(pid) {
		return false, 0, nil
	}
	return true, pid, nil
}

// alive (platform-specific) reports whether a process with the given pid exists
// and can be reached; see daemon_unix.go and daemon_windows.go.

// Start launches the daemon as a detached background process running
// `<exe> task schedule __run`. If it is already running, Start returns the
// existing pid with started=false.
func Start(exe string) (pid int, started bool, err error) {
	if running, p, err := Running(); err != nil {
		return 0, false, err
	} else if running {
		return p, false, nil
	}

	// Resolve the config dir to an absolute path now and pin the daemon to it
	// (below, via cmd.Env). Otherwise a relative EnvConfigDir would be re-resolved
	// against whatever working directory the daemon happens to inherit, so the
	// long-lived daemon could read/write a different schedules.json than the CLI.
	dir, err := ConfigDir()
	if err != nil {
		return 0, false, err
	}

	logPath, err := LogPath()
	if err != nil {
		return 0, false, err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, false, fmt.Errorf("open daemon log: %w", err)
	}
	defer func() { _ = logFile.Close() }()

	cmd := exec.Command(exe, "task", "schedule", "__run") //nolint:gosec // exe is our own os.Executable()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	cmd.Env = withConfigDir(os.Environ(), dir) // pin the daemon to the resolved absolute config dir
	cmd.SysProcAttr = detachAttr()             // detach from the controlling terminal/console

	if err := cmd.Start(); err != nil {
		return 0, false, fmt.Errorf("start daemon: %w", err)
	}
	pid = cmd.Process.Pid
	if err := writePid(pid); err != nil {
		// The daemon is already running but unreachable (Stop relies on the pid
		// file). Kill it so we don't leak an untracked background process.
		_ = cmd.Process.Kill()
		return 0, false, err
	}
	_ = cmd.Process.Release() // detached; don't reap
	return pid, true, nil
}

// Stop signals the running daemon to shut down. It returns stopped=false (and
// pid 0) when no daemon is running.
//
// On success the pid file is intentionally NOT removed here: on Unix the daemon
// removes its own pid file on exit (see Run's deferred removePid). Removing it
// eagerly would open a race where a daemon started before the old one finishes
// exiting gets its fresh pid file deleted by the old daemon's deferred cleanup,
// leaving two daemons running untracked. (On Windows terminate uses Kill, which
// skips deferred cleanup, so the pid file lingers as stale until the next
// Start/Stop reclaims it — harmless, since Running treats a dead pid as gone.)
func Stop() (stopped bool, pid int, err error) {
	running, p, err := Running()
	if err != nil {
		return false, 0, err
	}
	if !running {
		clearStalePid() // the process is gone; drop any leftover pid file
		return false, 0, nil
	}
	if err := terminate(p); err != nil {
		return false, p, fmt.Errorf("signal process %d: %w", p, err)
	}
	return true, p, nil
}

// ─── run loop ──────────────────────────────────────────────────────────────────

// Run is the daemon loop. It re-reads the config each tick, snapshots due
// entries via snap, persists run state, and notifies (when notify is non-nil and
// the entry opted in) if there is pending work. It returns when ctx is cancelled
// — the process is wired to cancel ctx on SIGTERM/SIGINT.
func Run(ctx context.Context, snap SnapshotFunc, notify Notifier) error {
	defer func() { _ = removePid() }()
	if err := writePid(os.Getpid()); err != nil {
		return err
	}
	logf("daemon started (pid %d)", os.Getpid())
	for {
		wait := tick(ctx, snap, notify)
		select {
		case <-ctx.Done():
			logf("daemon stopping (pid %d)", os.Getpid())
			return nil
		case <-time.After(wait):
		}
	}
}

// tick runs one pass over the config and returns how long to sleep before the
// next pass (capped at maxPoll so config edits are picked up promptly).
func tick(ctx context.Context, snap SnapshotFunc, notify Notifier) time.Duration {
	cfg, err := Load()
	if err != nil {
		logf("load config: %v", err)
		return maxPoll
	}

	now := time.Now()
	wait := maxPoll
	changed := false

	for i := range cfg.Entries {
		// Stop launching new snapshots once shutdown has begun, so SIGTERM takes
		// effect promptly even with many registered repos.
		if ctx.Err() != nil {
			break
		}
		e := &cfg.Entries[i]
		d, derr := e.ParsedInterval()
		if derr != nil {
			if e.LastError != derr.Error() {
				e.LastError = derr.Error()
				changed = true
			}
			continue
		}
		if e.LastRun != nil && now.Sub(*e.LastRun) < d {
			if remain := d - now.Sub(*e.LastRun); remain < wait {
				wait = remain
			}
			continue
		}
		if runEntry(ctx, e, snap, notify) {
			changed = true
		}
		if d < wait {
			wait = d
		}
	}

	if changed {
		if err := saveRunState(cfg.Entries); err != nil {
			logf("save run state: %v", err)
		}
	}

	if wait < time.Second {
		wait = time.Second
	}
	if wait > maxPoll {
		wait = maxPoll
	}
	return wait
}

// runEntry takes one snapshot for e, updating its run state in place. It
// reports whether e's persisted state changed and so should be saved.
//
// RunCount counts successful snapshots: it is incremented only after the
// snapshot is fetched and written, so a failed attempt is recorded in LastError
// without inflating the count. A snapshot aborted by context cancellation (the
// daemon shutting down) is not recorded as a failure and reports no change.
func runEntry(ctx context.Context, e *Entry, snap SnapshotFunc, notify Notifier) (changed bool) {
	data, summary, err := snap(ctx, *e)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		// Genuine cancellation (daemon shutting down) — don't record it as a
		// failure. A real error that merely coincides with shutdown is still
		// recorded below, so we don't lose it.
		logf("%s: snapshot cancelled (shutting down)", e.Repo)
		return false
	}

	t := time.Now()
	e.LastRun = &t
	if err != nil {
		e.LastError = err.Error()
		logf("%s: snapshot failed: %v", e.Repo, err)
		return true
	}
	if werr := writeSnapshot(e.GitDir, data); werr != nil {
		e.LastError = werr.Error()
		logf("%s: write snapshot: %v", e.Repo, werr)
		return true
	}
	e.RunCount++
	e.LastError = ""
	logf("%s: snapshot ok (review-requested=%d authored=%d)", e.Repo, summary.ReviewRequested, summary.Authored)

	if e.Notify && notify != nil && summary.ReviewRequested+summary.Authored > 0 {
		title := fmt.Sprintf("gh-wheel: %s", e.Repo)
		msg := fmt.Sprintf("👀 Review requested: %d / 📤 Authored: %d", summary.ReviewRequested, summary.Authored)
		if nerr := notify(title, msg); nerr != nil {
			logf("%s: notify failed: %v", e.Repo, nerr)
		}
	}
	return true
}

// saveRunState persists the daemon's per-entry run statistics (LastRun,
// RunCount, LastError) without clobbering edits made concurrently by
// `schedule add`/`remove`. It re-reads schedules.json and copies run state onto
// entries that still exist (matched by repo), so a repo the CLI added or
// removed since this tick began is preserved rather than overwritten by the
// snapshot the daemon loaded at the start of the tick.
//
// The remaining window — between this re-read and the atomic rename in Save —
// is small; a cross-process file lock would close it entirely but is not
// warranted for a single-user daemon writing infrequently.
func saveRunState(updated []Entry) error {
	cur, err := Load()
	if err != nil {
		return err
	}
	for _, u := range updated {
		if e, _ := cur.Find(u.Repo); e != nil {
			e.LastRun = u.LastRun
			e.RunCount = u.RunCount
			e.LastError = u.LastError
		}
	}
	return cur.Save()
}

// writeSnapshot writes data to <gitDir>/my-tasks/current.json atomically.
func writeSnapshot(gitDir string, data []byte) error {
	if gitDir == "" {
		return errors.New("entry has no git_dir; re-run schedule add inside the repository")
	}
	dir := filepath.Join(gitDir, "my-tasks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	out := filepath.Join(dir, "current.json")
	tmp := out + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, out); err != nil {
		return fmt.Errorf("rename %s: %w", out, err)
	}
	return nil
}

// ─── pid file + logging ─────────────────────────────────────────────────────────

// withConfigDir returns env with EnvConfigDir overridden to dir. exec uses the
// last occurrence of a duplicated key, so appending is enough to pin a spawned
// daemon to dir regardless of any inherited (possibly relative) value.
func withConfigDir(env []string, dir string) []string {
	return append(env, EnvConfigDir+"="+dir)
}

func writePid(pid int) error {
	p, err := PidPath()
	if err != nil {
		return err
	}
	if err := os.WriteFile(p, []byte(strconv.Itoa(pid)+"\n"), 0o644); err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}
	return nil
}

// removePid deletes the pid file only if it still names the current process.
// This is the daemon's own cleanup (deferred in Run): guarding on the pid means
// that if a newer daemon has already overwritten the file with its own pid, the
// exiting daemon won't delete the newcomer's pid file out from under it.
func removePid() error {
	p, err := PidPath()
	if err != nil {
		return err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read pid file: %w", err)
	}
	pid, perr := strconv.Atoi(strings.TrimSpace(string(b)))
	if perr != nil || pid != os.Getpid() {
		return nil // not ours (or unreadable) — leave it for the owner/Stop
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove pid file: %w", err)
	}
	return nil
}

// clearStalePid removes the pid file unconditionally. Stop calls it only after
// Running has confirmed the recorded process is gone, so the file is stale.
// Note: there is no cross-process lock, so a Start racing in the narrow window
// after Running() and before this os.Remove could have its fresh pid file
// deleted; this is acceptable for a single-user daemon that is rarely started
// from two terminals at the exact same moment.
func clearStalePid() {
	p, err := PidPath()
	if err != nil || p == "" {
		return
	}
	_ = os.Remove(p)
}

// logf writes a timestamped line to stdout, which the detached daemon has
// redirected to daemon.log.
func logf(format string, args ...any) {
	fmt.Printf("%s "+format+"\n", append([]any{time.Now().Format(time.RFC3339)}, args...)...)
}
