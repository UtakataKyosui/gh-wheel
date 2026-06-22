package schedule

import (
	"os"
	"path/filepath"
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
	snap := func(e Entry) ([]byte, Summary, error) {
		calls++
		return []byte(`{"kind":"task_result","prs":[]}`), Summary{Repo: e.Repo, ReviewRequested: 2, Authored: 1}, nil
	}

	wait := tick(snap, nil)
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
	tick(snap, nil)
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
	snap := func(_ Entry) ([]byte, Summary, error) {
		return nil, Summary{}, errFake
	}
	tick(snap, nil)
	got, _ := Load()
	if got.Entries[0].LastError == "" {
		t.Error("snapshot error should be recorded in LastError")
	}
}

var errFake = &fakeErr{}

type fakeErr struct{}

func (*fakeErr) Error() string { return "boom" }

// neverNotify documents the expected Notifier signature; kept for clarity.
var _ Notifier = func(_, _ string) error { return nil }

// dueRespectsInterval guards against off-by-one regressions in NextRun math.
func TestTickWaitRespectsInterval(t *testing.T) {
	t.Setenv("GH_WHEEL_CONFIG_DIR", t.TempDir())
	last := time.Now()
	cfg := &Config{Entries: []Entry{{Repo: "a/b", GitDir: t.TempDir(), Interval: "2m", LastRun: &last}}}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	wait := tick(func(Entry) ([]byte, Summary, error) { return nil, Summary{}, nil }, nil)
	// Not due (just ran); wait should be capped at maxPoll, well under the 2m interval.
	if wait > maxPoll {
		t.Errorf("wait = %v, want <= %v", wait, maxPoll)
	}
}
