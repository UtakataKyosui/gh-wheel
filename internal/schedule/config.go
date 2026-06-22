// Package schedule persists the set of repositories the gh-wheel snapshot
// daemon watches and provides the daemon's process management and run loop.
//
// Configuration lives under the per-user config directory — honouring
// GH_WHEEL_CONFIG_DIR, then XDG_CONFIG_HOME, then ~/.config — at
// gh-wheel/schedules.json, alongside the daemon's pid and log files. The daemon
// itself needs no cron or launchd: `gh wheel task schedule add` registers a repo
// and starts a self-contained background process that snapshots each repo's task
// list to <repo>/.git/my-tasks/current.json on its configured interval.
package schedule

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SchemaVersion is the schedules.json format version.
const SchemaVersion = "v1"

// MinInterval is the smallest snapshot interval allowed, to avoid hammering the
// GitHub API (and tripping secondary rate limits).
const MinInterval = time.Minute

// Entry is a single watched repository and its snapshot settings.
type Entry struct {
	Repo          string     `json:"repo"`    // "owner/name"
	GitDir        string     `json:"git_dir"` // absolute path to the repo's .git directory
	Interval      string     `json:"interval"`
	State         string     `json:"state"`
	AuthorOnly    bool       `json:"author_only"`
	ReviewOnly    bool       `json:"review_only"`
	IncludeDrafts bool       `json:"include_drafts"`
	WithReviews   bool       `json:"with_reviews"`
	Notify        bool       `json:"notify"`
	AddedAt       time.Time  `json:"added_at"`
	LastRun       *time.Time `json:"last_run,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
	RunCount      int        `json:"run_count"`
}

// ParsedInterval returns the entry's interval as a time.Duration.
func (e Entry) ParsedInterval() (time.Duration, error) {
	return time.ParseDuration(e.Interval)
}

// NextRun returns when the entry is next due for a snapshot. A never-run entry
// (LastRun nil) returns the zero time, meaning "due now".
func (e Entry) NextRun() time.Time {
	if e.LastRun == nil {
		return time.Time{}
	}
	d, err := e.ParsedInterval()
	if err != nil {
		return *e.LastRun
	}
	return e.LastRun.Add(d)
}

// Config is the persisted schedules.json document.
type Config struct {
	SchemaVersion string  `json:"schema_version"`
	Entries       []Entry `json:"entries"`
}

// Find returns a pointer to the entry for repo and its index, or (nil, -1).
func (c *Config) Find(repo string) (*Entry, int) {
	for i := range c.Entries {
		if c.Entries[i].Repo == repo {
			return &c.Entries[i], i
		}
	}
	return nil, -1
}

// Upsert replaces the existing entry for e.Repo (preserving its accumulated run
// statistics) or appends it. It reports whether an existing entry was replaced.
func (c *Config) Upsert(e Entry) bool {
	if existing, i := c.Find(e.Repo); i >= 0 {
		e.AddedAt = existing.AddedAt
		e.LastRun = existing.LastRun
		e.LastError = existing.LastError
		e.RunCount = existing.RunCount
		c.Entries[i] = e
		return true
	}
	c.Entries = append(c.Entries, e)
	return false
}

// Remove deletes the entry for repo, reporting whether one was removed.
func (c *Config) Remove(repo string) bool {
	if _, i := c.Find(repo); i >= 0 {
		c.Entries = append(c.Entries[:i], c.Entries[i+1:]...)
		return true
	}
	return false
}

// Save writes the config to schedules.json atomically (temp file + rename).
func (c *Config) Save() error {
	p, err := SchedulesPath()
	if err != nil {
		return err
	}
	if c.SchemaVersion == "" {
		c.SchemaVersion = SchemaVersion
	}
	if c.Entries == nil {
		c.Entries = []Entry{}
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal schedules: %w", err)
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, p); err != nil {
		return fmt.Errorf("rename %s: %w", p, err)
	}
	return nil
}

// Load reads schedules.json, returning an empty config when the file is absent.
func Load() (*Config, error) {
	p, err := SchedulesPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{SchemaVersion: SchemaVersion, Entries: []Entry{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", p, err)
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", p, err)
	}
	if c.SchemaVersion == "" {
		c.SchemaVersion = SchemaVersion
	}
	if c.Entries == nil {
		c.Entries = []Entry{}
	}
	return &c, nil
}

// ValidateInterval parses s as a Go duration and enforces the minimum interval.
func ValidateInterval(s string) (time.Duration, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid interval %q: must be a Go duration like 5m, 30s, 1h", s)
	}
	if d < MinInterval {
		return 0, fmt.Errorf("interval %s is too short; minimum is %s", d, MinInterval)
	}
	return d, nil
}

// ─── paths ──────────────────────────────────────────────────────────────────

// ConfigDir resolves (and creates) the gh-wheel configuration directory.
func ConfigDir() (string, error) {
	if d := os.Getenv("GH_WHEEL_CONFIG_DIR"); d != "" {
		return d, ensureDir(d)
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	d := filepath.Join(base, "gh-wheel")
	return d, ensureDir(d)
}

// SchedulesPath returns the path to schedules.json.
func SchedulesPath() (string, error) { return configFile("schedules.json") }

// PidPath returns the path to the daemon pid file.
func PidPath() (string, error) { return configFile("daemon.pid") }

// LogPath returns the path to the daemon log file.
func LogPath() (string, error) { return configFile("daemon.log") }

func configFile(name string) (string, error) {
	d, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, name), nil
}

func ensureDir(d string) error {
	if err := os.MkdirAll(d, 0o755); err != nil {
		return fmt.Errorf("create config dir %s: %w", d, err)
	}
	return nil
}
