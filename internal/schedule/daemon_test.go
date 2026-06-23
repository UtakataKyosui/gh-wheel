package schedule

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAlive(t *testing.T) {
	if !alive(os.Getpid()) {
		t.Error("alive(self) should be true")
	}
	if alive(0) {
		t.Error("alive(0) should be false")
	}
	if alive(-1) {
		t.Error("alive(-1) should be false")
	}
}

func TestRunningStates(t *testing.T) {
	t.Setenv("GH_WHEEL_CONFIG_DIR", t.TempDir())

	// No pid file.
	if running, _, err := Running(); err != nil || running {
		t.Errorf("Running with no pid file = (%v, _, %v), want (false, _, nil)", running, err)
	}

	// Our own (live) pid.
	if err := writePid(os.Getpid()); err != nil {
		t.Fatalf("writePid: %v", err)
	}
	running, pid, err := Running()
	if err != nil || !running || pid != os.Getpid() {
		t.Errorf("Running with own pid = (%v, %d, %v), want (true, %d, nil)", running, pid, err, os.Getpid())
	}

	// Corrupt pid file → treated as not running.
	p, _ := PidPath()
	if err := os.WriteFile(p, []byte("not-a-number\n"), 0o644); err != nil {
		t.Fatalf("write corrupt pid: %v", err)
	}
	if running, _, err := Running(); err != nil || running {
		t.Errorf("Running with corrupt pid file = (%v, _, %v), want (false, _, nil)", running, err)
	}
}

func TestStopWhenNotRunning(t *testing.T) {
	t.Setenv("GH_WHEEL_CONFIG_DIR", t.TempDir())
	stopped, pid, err := Stop()
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if stopped || pid != 0 {
		t.Errorf("Stop when not running = (%v, %d), want (false, 0)", stopped, pid)
	}
}

func TestWriteSnapshot(t *testing.T) {
	gitDir := t.TempDir()
	data := []byte(`{"kind":"task_result"}`)
	if err := writeSnapshot(gitDir, data); err != nil {
		t.Fatalf("writeSnapshot: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(gitDir, "my-tasks", "current.json"))
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("snapshot = %q, want %q", got, data)
	}
}

func TestWriteSnapshotNoGitDir(t *testing.T) {
	if err := writeSnapshot("", []byte("x")); err == nil {
		t.Error("writeSnapshot with empty gitDir should error")
	}
}

func TestTickRunsDueEntry(t *testing.T) {
	t.Setenv("GH_WHEEL_CONFIG_DIR", t.TempDir())
	gitDir := t.TempDir()

	cfg := &Config{SchemaVersion: SchemaVersion, Entries: []Entry{
		{Repo: "a/b", GitDir: gitDir, Interval: "5m", State: "open", Notify: false},
	}}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var calls int
	snap := func(_ context.Context, e Entry) ([]byte, Summary, error) {
		calls++
		return []byte(`{"kind":"task_result","prs":[]}`), Summary{Repo: e.Repo, ReviewRequested: 2, Authored: 1}, nil
	}

	wait := tick(context.Background(), snap, nil)
	if calls != 1 {
		t.Fatalf("snap called %d times, want 1 (entry was due)", calls)
	}
	if wait <= 0 || wait > maxPoll {
		t.Errorf("wait = %v, want in (0, %v]", wait, maxPoll)
	}

	// Snapshot file written.
	if _, err := os.Stat(filepath.Join(gitDir, "my-tasks", "current.json")); err != nil {
		t.Errorf("snapshot file not written: %v", err)
	}

	// Run state persisted.
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Entries[0].RunCount != 1 || got.Entries[0].LastRun == nil {
		t.Errorf("run state not persisted: RunCount=%d LastRun=%v", got.Entries[0].RunCount, got.Entries[0].LastRun)
	}

	// A second tick right away should NOT re-run (not due yet).
	calls = 0
	tick(context.Background(), snap, nil)
	if calls != 0 {
		t.Errorf("snap called %d times on second tick, want 0 (not due)", calls)
	}
}

func TestTickRecordsSnapshotError(t *testing.T) {
	t.Setenv("GH_WHEEL_CONFIG_DIR", t.TempDir())
	cfg := &Config{Entries: []Entry{{Repo: "a/b", GitDir: t.TempDir(), Interval: "5m"}}}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	snap := func(_ context.Context, _ Entry) ([]byte, Summary, error) {
		return nil, Summary{}, errFake
	}
	tick(context.Background(), snap, nil)
	got, _ := Load()
	if got.Entries[0].LastError == "" {
		t.Error("snapshot error should be recorded in LastError")
	}
	if got.Entries[0].RunCount != 0 {
		t.Errorf("RunCount = %d, want 0: a failed snapshot must not count as a run", got.Entries[0].RunCount)
	}
}

var errFake = &fakeErr{}

type fakeErr struct{}

func (*fakeErr) Error() string { return "boom" }

// neverNotify documents the expected Notifier signature; kept for clarity.
var _ Notifier = func(_, _ string) error { return nil }

// _ documents the expected SnapshotFunc signature (now context-aware).
var _ SnapshotFunc = func(context.Context, Entry) ([]byte, Summary, error) { return nil, Summary{}, nil }

// TestTickCancelledContextSkipsSnapshot verifies the run loop stops launching
// snapshots once the daemon's context is cancelled (graceful SIGTERM shutdown).
func TestTickCancelledContextSkipsSnapshot(t *testing.T) {
	t.Setenv("GH_WHEEL_CONFIG_DIR", t.TempDir())
	cfg := &Config{Entries: []Entry{{Repo: "a/b", GitDir: t.TempDir(), Interval: "5m"}}}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before the tick begins

	var calls int
	snap := func(context.Context, Entry) ([]byte, Summary, error) {
		calls++
		return nil, Summary{}, nil
	}
	tick(ctx, snap, nil)
	if calls != 0 {
		t.Errorf("snap called %d times under a cancelled context, want 0", calls)
	}
}

// TestSaveRunStatePreservesConcurrentEdits verifies the daemon's run-state save
// merges onto the latest on-disk config instead of clobbering an add/remove the
// CLI performed mid-tick.
func TestSaveRunStatePreservesConcurrentEdits(t *testing.T) {
	t.Setenv("GH_WHEEL_CONFIG_DIR", t.TempDir())

	// The daemon's view at the start of a tick: a single repo.
	base := &Config{Entries: []Entry{{Repo: "a/b", Interval: "5m"}}}
	if err := base.Save(); err != nil {
		t.Fatalf("Save base: %v", err)
	}

	// It computes updated run state for a/b.
	now := time.Now()
	updated := []Entry{{Repo: "a/b", Interval: "5m", LastRun: &now, RunCount: 1}}

	// Meanwhile the CLI edits a/b's interval and registers a second repo.
	cur, _ := Load()
	cur.Entries[0].Interval = "10m"
	cur.Upsert(Entry{Repo: "c/d", Interval: "1m"})
	if err := cur.Save(); err != nil {
		t.Fatalf("CLI Save: %v", err)
	}

	// The daemon persists run state via merge — the CLI's edits must survive.
	if err := saveRunState(updated); err != nil {
		t.Fatalf("saveRunState: %v", err)
	}

	got, _ := Load()
	if len(got.Entries) != 2 {
		t.Fatalf("entries = %d, want 2 (the CLI's add must survive)", len(got.Entries))
	}
	ab, _ := got.Find("a/b")
	if ab == nil || ab.RunCount != 1 || ab.LastRun == nil {
		t.Errorf("a/b run state not merged: %+v", ab)
	}
	if ab != nil && ab.Interval != "10m" {
		t.Errorf("a/b interval = %q, want 10m (the CLI's edit must survive)", ab.Interval)
	}
	if cd, _ := got.Find("c/d"); cd == nil {
		t.Error("c/d (added by the CLI mid-tick) was clobbered")
	}
}

// TestRemovePidOnlyRemovesOwn verifies the daemon's deferred cleanup only
// deletes the pid file when it still names the current process, so a newer
// daemon's pid file is never deleted out from under it.
func TestRemovePidOnlyRemovesOwn(t *testing.T) {
	t.Setenv("GH_WHEEL_CONFIG_DIR", t.TempDir())
	p, _ := PidPath()

	// A pid file owned by another process must be left alone.
	if err := os.WriteFile(p, []byte(strconv.Itoa(os.Getpid()+1)+"\n"), 0o644); err != nil {
		t.Fatalf("write foreign pid: %v", err)
	}
	if err := removePid(); err != nil {
		t.Fatalf("removePid (foreign): %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Error("removePid deleted a pid file it does not own")
	}

	// Our own pid file is removed.
	if err := writePid(os.Getpid()); err != nil {
		t.Fatalf("writePid: %v", err)
	}
	if err := removePid(); err != nil {
		t.Fatalf("removePid (own): %v", err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Errorf("removePid did not remove our own pid file (stat err=%v)", err)
	}
}

// TestWithConfigDirPinsDaemon verifies the daemon env override wins over an
// inherited (possibly relative) EnvConfigDir, so a spawned daemon resolves the
// same absolute config dir as its spawner.
func TestWithConfigDirPinsDaemon(t *testing.T) {
	env := withConfigDir([]string{EnvConfigDir + "=rel-cfg", "PATH=/x"}, "/abs/cfg")
	// The child process uses the last occurrence of a duplicated key.
	var last string
	for _, e := range env {
		if strings.HasPrefix(e, EnvConfigDir+"=") {
			last = e
		}
	}
	if last != EnvConfigDir+"=/abs/cfg" {
		t.Errorf("effective override = %q, want %s=/abs/cfg", last, EnvConfigDir)
	}
}

// TestStopClearsStalePidFile verifies Stop drops a pid file whose process is
// gone (rather than leaving it to mislead a later Start).
func TestStopClearsStalePidFile(t *testing.T) {
	t.Setenv("GH_WHEEL_CONFIG_DIR", t.TempDir())
	p, _ := PidPath()
	if err := os.WriteFile(p, []byte("999999\n"), 0o644); err != nil { // dead/nonexistent pid
		t.Fatalf("write stale pid: %v", err)
	}
	stopped, pid, err := Stop()
	if err != nil || stopped || pid != 0 {
		t.Fatalf("Stop with stale pid = (%v, %d, %v), want (false, 0, nil)", stopped, pid, err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Errorf("Stop did not clear the stale pid file (stat err=%v)", err)
	}
}

// dueRespectsInterval guards against off-by-one regressions in NextRun math.
func TestTickWaitRespectsInterval(t *testing.T) {
	t.Setenv("GH_WHEEL_CONFIG_DIR", t.TempDir())
	last := time.Now()
	cfg := &Config{Entries: []Entry{{Repo: "a/b", GitDir: t.TempDir(), Interval: "2m", LastRun: &last}}}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	wait := tick(context.Background(), func(context.Context, Entry) ([]byte, Summary, error) { return nil, Summary{}, nil }, nil)
	// Not due (just ran); wait should be capped at maxPoll, well under the 2m interval.
	if wait > maxPoll {
		t.Errorf("wait = %v, want <= %v", wait, maxPoll)
	}
}
