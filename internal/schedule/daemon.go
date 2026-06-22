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
	"syscall"
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
// this package stays free of GitHub specifics (and is unit-testable).
type SnapshotFunc func(e Entry) (data []byte, summary Summary, err error)

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

// alive reports whether a process with the given pid exists and is signalable.
func alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// Start launches the daemon as a detached background process running
// `<exe> task schedule __run`. If it is already running, Start returns the
// existing pid with started=false.
func Start(exe string) (pid int, started bool, err error) {
	if running, p, err := Running(); err != nil {
		return 0, false, err
	} else if running {
		return p, false, nil
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
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // detach from the controlling terminal

	if err := cmd.Start(); err != nil {
		return 0, false, fmt.Errorf("start daemon: %w", err)
	}
	pid = cmd.Process.Pid
	if err := writePid(pid); err != nil {
		return 0, false, err
	}
	_ = cmd.Process.Release() // detached via setsid; don't reap
	return pid, true, nil
}

// Stop sends SIGTERM to the running daemon. It returns stopped=false (and pid 0)
// when no daemon is running.
func Stop() (stopped bool, pid int, err error) {
	running, p, err := Running()
	if err != nil {
		return false, 0, err
	}
	if !running {
		_ = removePid() // clear any stale file
		return false, 0, nil
	}
	proc, err := os.FindProcess(p)
	if err != nil {
		return false, p, fmt.Errorf("find process %d: %w", p, err)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return false, p, fmt.Errorf("signal process %d: %w", p, err)
	}
	_ = removePid()
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
		wait := tick(snap, notify)
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
func tick(snap SnapshotFunc, notify Notifier) time.Duration {
	cfg, err := Load()
	if err != nil {
		logf("load config: %v", err)
		return maxPoll
	}

	now := time.Now()
	wait := maxPoll
	changed := false

	for i := range cfg.Entries {
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
		runEntry(e, snap, notify)
		changed = true
		if d < wait {
			wait = d
		}
	}

	if changed {
		if err := cfg.Save(); err != nil {
			logf("save config: %v", err)
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

// runEntry takes one snapshot for e, updating its run state in place.
func runEntry(e *Entry, snap SnapshotFunc, notify Notifier) {
	t := time.Now()
	e.LastRun = &t
	e.RunCount++

	data, summary, err := snap(*e)
	if err != nil {
		e.LastError = err.Error()
		logf("%s: snapshot failed: %v", e.Repo, err)
		return
	}
	if err := writeSnapshot(e.GitDir, data); err != nil {
		e.LastError = err.Error()
		logf("%s: write snapshot: %v", e.Repo, err)
		return
	}
	e.LastError = ""
	logf("%s: snapshot ok (review-requested=%d authored=%d)", e.Repo, summary.ReviewRequested, summary.Authored)

	if e.Notify && notify != nil && summary.ReviewRequested+summary.Authored > 0 {
		title := fmt.Sprintf("gh-wheel: %s", e.Repo)
		msg := fmt.Sprintf("👀 レビュー依頼 %d / 📤 自分の PR %d", summary.ReviewRequested, summary.Authored)
		if nerr := notify(title, msg); nerr != nil {
			logf("%s: notify failed: %v", e.Repo, nerr)
		}
	}
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

func removePid() error {
	p, err := PidPath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove pid file: %w", err)
	}
	return nil
}

// logf writes a timestamped line to stdout, which the detached daemon has
// redirected to daemon.log.
func logf(format string, args ...any) {
	fmt.Printf("%s "+format+"\n", append([]any{time.Now().Format(time.RFC3339)}, args...)...)
}
