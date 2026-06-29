### `gh wheel task`

Manage your GitHub tasks (PRs and Issues)

Browse and operate on PRs and Issues you are involved in as author or reviewer.

```
gh wheel task [flags]
```

フラグ:

- `-a, --author-only` — Show only PRs where you are the author
- `-d, --include-drafts` — Include draft PRs (default "true")
- `--issues-only` — Show only Issues (implies --with-issues)
- `-r, --review-only` — Show only PRs where review is requested from you
- `-s, --state` — Filter by state: open, closed, all (default "open")
- `-I, --with-issues` — Include Issues assigned to you
- `--with-reviews` — Fetch review status for each PR (slower)

#### `gh wheel task close`

Close a PR or Issue by number

Close the PR or Issue with the given number.

By default the command prints the item's title, state, and URL, then asks you
to re-enter the number as a confirmation before closing.  Pass --json to skip
the confirmation and close immediately.

```
gh wheel task close <N>
```

#### `gh wheel task next`

Find an approachable unstarted Issue and self-assign

Find an open, unassigned Issue that is ready to start and assign it to you.

"Ready to start" means: open, unassigned, not labelled epic/wontfix/invalid/
duplicate/question, and (unless --no-blockers) not already linked to a PR and
not an aggregator with open sub-issues. Candidates are ranked, preferring
"good first issue" / "help wanted" and de-prioritising "priority:low".

By default the top candidate is shown and you re-enter its number to confirm
before it is assigned. Pass [N] to target a specific Issue, --list to only show
candidates, --dry-run to preview, or --json/--yes to skip the confirmation.

```
gh wheel task next [N] [flags]
```

フラグ:

- `--label` — Restrict candidates to a label
- `--limit` — Maximum number of candidates to show (default "5")
- `--list` — List candidates without assigning
- `--no-blockers` — Skip blocker filtering (faster; no GraphQL queries)
- `--yes` — Skip the confirmation prompt before assigning

#### `gh wheel task schedule`

Periodically snapshot your tasks via a self-contained daemon

Run a background daemon that periodically snapshots your PR/Issue list to
<repo>/.git/my-tasks/current.json — no cron or launchd required.

Register the current repository with 'schedule add' (which starts the daemon if
needed), then 'list'/'status' to inspect it and 'stop' to halt it. With --notify,
each snapshot also raises a desktop notification summarising pending work.

##### `gh wheel task schedule add`

Register the current repository with the snapshot daemon

Register the current repository for periodic snapshots and (unless --no-start)
start the daemon. Re-running 'add' for the same repository updates its settings
while preserving its run statistics.

```
gh wheel task schedule add [flags]
```

フラグ:

- `-a, --author-only` — Snapshot only PRs you authored
- `-d, --include-drafts` — Include draft PRs (default "true")
- `--interval` — Snapshot interval as a Go duration (e.g. 5m, 30s, 1h; minimum 1m) (default "5m")
- `--no-start` — Do not start the daemon automatically
- `--notify` — Show a desktop notification after each snapshot
- `-r, --review-only` — Snapshot only PRs where review is requested from you
- `-s, --state` — Filter by state: open, closed, all (default "open")
- `--with-reviews` — Fetch review status for each PR (slower)

##### `gh wheel task schedule list`

List watched repositories and the daemon status

```
gh wheel task schedule list
```

##### `gh wheel task schedule remove`

Unregister the current repository from the snapshot daemon

```
gh wheel task schedule remove
```

##### `gh wheel task schedule start`

Start the snapshot daemon

```
gh wheel task schedule start
```

##### `gh wheel task schedule status`

Show the daemon pid and per-repo run counts

```
gh wheel task schedule status
```

##### `gh wheel task schedule stop`

Stop the snapshot daemon (SIGTERM)

```
gh wheel task schedule stop
```

#### `gh wheel task time`

Show estimated work time breakdown for a PR by session

Analyze PR commits and estimate actual work time using a session-based algorithm.

A "session" is a continuous work block. Gaps longer than --session-gap minutes
between commits start a new session. Each session's start is estimated by
subtracting --lead-time minutes from the first commit (crediting work done
before committing). Single-commit sessions are guaranteed at least --min-session
minutes.

Example:
  gh wheel task time 42
  gh wheel task time 42 --tz Asia/Tokyo --json

```
gh wheel task time <PR> [flags]
```

フラグ:

- `--lead-time` — Minutes added before the first commit of each session (default "30")
- `--min-session` — Minimum session duration in minutes (default "15")
- `--session-gap` — Gap in minutes between commits that starts a new session (default "30")
- `--tz` — Timezone for date grouping (e.g. "Asia/Tokyo") (default "Local")

#### `gh wheel task today`

Plan today's tasks within a time budget

Build a read-only plan of what to work on today, fitted to a time budget.

It gathers everything on your plate — PRs awaiting your review, your own PRs
that are approved (ready to merge) or have changes requested, Issues assigned to
you, and (if budget remains) fresh approachable Issues — then ranks them by
priority:

  review > pr-merge > pr-fix > in-progress > new

Each item is given a per-category effort estimate and the budget is filled
greedily in priority order: items that fit go into the plan, the rest are
deferred. The single highest-priority item is always included even if it alone
exceeds the budget (so the plan is never empty when there is work).

This command is READ-ONLY: it never assigns, labels, or merges anything, so
--dry-run produces identical output. Honours -R/--json/--jq.

Examples:
  gh wheel task today
  gh wheel task today --budget 4h --json
  gh wheel task today --no-new --json

```
gh wheel task today [flags]
```

フラグ:

- `--budget` — Time budget for today (duration: 6h, 90m, 4h30m) (default "6h")
- `--issue-effort` — Estimated minutes per Issue (in-progress or new) (default "90")
- `--merge-effort` — Estimated minutes to merge an approved PR (default "10")
- `--no-blockers` — Skip blocker filtering for new Issues (faster; no GraphQL queries)
- `--no-new` — Do not include fresh approachable Issues
- `--pr-effort` — Estimated minutes to address changes requested on your PR (default "45")
- `--review-effort` — Estimated minutes per PR review (default "20")