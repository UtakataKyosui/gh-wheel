package schedule

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingReturnsEmpty(t *testing.T) {
	t.Setenv("GH_WHEEL_CONFIG_DIR", t.TempDir())
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", cfg.SchemaVersion, SchemaVersion)
	}
	if cfg.Entries == nil {
		t.Error("Entries should be an empty slice, not nil")
	}
	if len(cfg.Entries) != 0 {
		t.Errorf("len(Entries) = %d, want 0", len(cfg.Entries))
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("GH_WHEEL_CONFIG_DIR", t.TempDir())
	now := time.Now().UTC().Truncate(time.Second)
	cfg := &Config{SchemaVersion: SchemaVersion, Entries: []Entry{
		{Repo: "a/b", GitDir: "/tmp/a/.git", Interval: "5m", State: "open", IncludeDrafts: true, AddedAt: now, RunCount: 2, LastRun: &now},
	}}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Entries) != 1 || got.Entries[0].Repo != "a/b" {
		t.Fatalf("round trip entries = %+v", got.Entries)
	}
	if got.Entries[0].RunCount != 2 {
		t.Errorf("RunCount = %d, want 2", got.Entries[0].RunCount)
	}
	if got.Entries[0].LastRun == nil || !got.Entries[0].LastRun.Equal(now) {
		t.Errorf("LastRun = %v, want %v", got.Entries[0].LastRun, now)
	}
}

func TestUpsertInsertAndReplace(t *testing.T) {
	cfg := &Config{SchemaVersion: SchemaVersion, Entries: []Entry{}}

	if replaced := cfg.Upsert(Entry{Repo: "a/b", Interval: "5m"}); replaced {
		t.Error("first Upsert should insert, not replace")
	}
	if len(cfg.Entries) != 1 {
		t.Fatalf("len = %d, want 1", len(cfg.Entries))
	}

	// Simulate the daemon having run the entry.
	now := time.Now()
	cfg.Entries[0].LastRun = &now
	cfg.Entries[0].RunCount = 7

	if replaced := cfg.Upsert(Entry{Repo: "a/b", Interval: "10m"}); !replaced {
		t.Error("second Upsert should replace")
	}
	if len(cfg.Entries) != 1 {
		t.Fatalf("len = %d, want 1 after replace", len(cfg.Entries))
	}
	e := cfg.Entries[0]
	if e.Interval != "10m" {
		t.Errorf("Interval = %q, want 10m", e.Interval)
	}
	if e.RunCount != 7 || e.LastRun == nil {
		t.Errorf("Upsert should preserve run stats: RunCount=%d LastRun=%v", e.RunCount, e.LastRun)
	}
}

func TestFindAndRemove(t *testing.T) {
	cfg := &Config{Entries: []Entry{{Repo: "a/b"}, {Repo: "c/d"}}}
	if e, i := cfg.Find("c/d"); e == nil || i != 1 {
		t.Errorf("Find(c/d) = (%v, %d)", e, i)
	}
	if e, i := cfg.Find("x/y"); e != nil || i != -1 {
		t.Errorf("Find(x/y) = (%v, %d), want (nil, -1)", e, i)
	}
	if !cfg.Remove("a/b") {
		t.Error("Remove(a/b) should report true")
	}
	if cfg.Remove("a/b") {
		t.Error("Remove(a/b) twice should report false")
	}
	if len(cfg.Entries) != 1 || cfg.Entries[0].Repo != "c/d" {
		t.Errorf("after remove entries = %+v", cfg.Entries)
	}
}

func TestValidateInterval(t *testing.T) {
	if _, err := ValidateInterval("5m"); err != nil {
		t.Errorf("ValidateInterval(5m) unexpected err: %v", err)
	}
	if _, err := ValidateInterval("nonsense"); err == nil {
		t.Error("ValidateInterval(nonsense) should error")
	}
	if _, err := ValidateInterval("10s"); err == nil {
		t.Error("ValidateInterval(10s) should error: below minimum")
	}
}

func TestConfigDirResolvesRelativeToAbsolute(t *testing.T) {
	// Run from a scratch dir so the relative config dir is created there.
	t.Chdir(t.TempDir())
	t.Setenv("GH_WHEEL_CONFIG_DIR", "rel-cfg")

	d, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir: %v", err)
	}
	if !filepath.IsAbs(d) {
		t.Errorf("ConfigDir() = %q, want an absolute path", d)
	}
	if filepath.Base(d) != "rel-cfg" {
		t.Errorf("ConfigDir() = %q, want it to end in rel-cfg", d)
	}
}

func TestNextRun(t *testing.T) {
	var never Entry
	if !never.NextRun().IsZero() {
		t.Error("never-run entry NextRun should be zero (due now)")
	}
	last := time.Date(2026, 6, 22, 9, 0, 0, 0, time.UTC)
	e := Entry{Interval: "30m", LastRun: &last}
	want := last.Add(30 * time.Minute)
	if got := e.NextRun(); !got.Equal(want) {
		t.Errorf("NextRun = %v, want %v", got, want)
	}
}
