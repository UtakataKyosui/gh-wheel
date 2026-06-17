// Package worktime estimates developer work time from commit timestamps using a
// session-based algorithm. Gaps between commits that exceed SessionGap start a
// new session; each session's start is moved back by LeadTime to account for
// work done before the first commit.
package worktime

import (
	"sort"
	"time"
)

// Default parameter values (in minutes) exposed so callers can populate flags.
const (
	DefaultSessionGapMinutes = 30
	DefaultLeadTimeMinutes   = 30
	DefaultMinSessionMinutes = 15
)

// Params controls the session-based estimation algorithm.
type Params struct {
	SessionGap time.Duration  // commit gap that starts a new session
	LeadTime   time.Duration  // time subtracted from a session's first commit
	MinSession time.Duration  // floor for any single session's duration
	Location   *time.Location // timezone used for day boundary grouping
}

// DefaultParams returns sensible defaults.
func DefaultParams() Params {
	return Params{
		SessionGap: DefaultSessionGapMinutes * time.Minute,
		LeadTime:   DefaultLeadTimeMinutes * time.Minute,
		MinSession: DefaultMinSessionMinutes * time.Minute,
		Location:   time.UTC,
	}
}

// Session is an estimated continuous work block derived from one or more commits.
type Session struct {
	Start    time.Time     // first_commit − LeadTime
	End      time.Time     // last commit in the session
	Duration time.Duration // max(End−Start, MinSession)
}

// DaySummary aggregates sessions that fall on the same calendar day (in
// p.Location timezone).
type DaySummary struct {
	Date     string // "YYYY-MM-DD"
	Sessions []Session
	Total    time.Duration
}

// Calculate estimates work sessions from commit timestamps.
// Timestamps need not be sorted; Calculate sorts them internally.
func Calculate(commits []time.Time, p Params) []DaySummary {
	if len(commits) == 0 {
		return []DaySummary{}
	}

	sorted := make([]time.Time, len(commits))
	copy(sorted, commits)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Before(sorted[j]) })

	sessions := splitIntoSessions(sorted, p)
	return groupByDay(sessions, p)
}

func splitIntoSessions(commits []time.Time, p Params) []Session {
	var sessions []Session
	start := commits[0]
	prev := commits[0]

	for i := 1; i < len(commits); i++ {
		if commits[i].Sub(prev) > p.SessionGap {
			sessions = append(sessions, buildSession(start, prev, p))
			start = commits[i]
		}
		prev = commits[i]
	}
	return append(sessions, buildSession(start, prev, p))
}

func buildSession(first, last time.Time, p Params) Session {
	s := Session{
		Start: first.Add(-p.LeadTime),
		End:   last,
	}
	d := s.End.Sub(s.Start)
	if d < p.MinSession {
		d = p.MinSession
	}
	s.Duration = d
	return s
}

func groupByDay(sessions []Session, p Params) []DaySummary {
	loc := p.Location
	if loc == nil {
		loc = time.UTC
	}

	seen := []string{}
	byDay := map[string][]Session{}

	for _, s := range sessions {
		// Group by the first commit time (Start + LeadTime) so that sessions
		// starting just before midnight don't get attributed to the previous day.
		firstCommit := s.Start.Add(p.LeadTime)
		day := firstCommit.In(loc).Format("2006-01-02")
		if _, exists := byDay[day]; !exists {
			seen = append(seen, day)
		}
		byDay[day] = append(byDay[day], s)
	}
	sort.Strings(seen)

	summaries := make([]DaySummary, 0, len(seen))
	for _, day := range seen {
		var total time.Duration
		for _, s := range byDay[day] {
			total += s.Duration
		}
		summaries = append(summaries, DaySummary{
			Date:     day,
			Sessions: byDay[day],
			Total:    total,
		})
	}
	return summaries
}
