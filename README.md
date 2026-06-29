# gh-wheel

A unified `gh` extension for Issue-Driven development, supporting two complementary workflows:

**As a developer** — Check an Issue, open a PR, respond to review comments, and report fixes back to reviewers.

**As a reviewer** — Pick up a review request, generate an AI-assisted review, and post structured comments to the PR.

```
gh wheel task     — browse and manage your PRs and Issues
gh wheel graph    — visualize Issue/PR dependency graphs
gh wheel monitor  — watch multiple repos in a live TUI
gh wheel review   — AI-assisted code review workflows
gh wheel okr      — compute GitHub activity metrics for OKR key results
gh wheel describe — print command schema as JSON (for AI agents)
```

## Workflows

### Developer workflow

```bash
# 1. Check what Issues and PRs need your attention
gh wheel task

# 2. Visualize Issue dependencies before starting
gh wheel graph --issue 42

# 3. After pushing a PR, check for unresolved review threads
gh wheel review threads 42

# 4. Reply to a review comment to report your fix
gh wheel review reply 42 --comment-id 123456789 --body "Fixed in latest commit."
```

### Reviewer workflow

```bash
# 1. List PRs where review is requested from you
gh wheel task -r

# 2. Generate a Markdown prompt for AI-assisted review
gh wheel review prompt 42

# 3. Feed the prompt to an AI, save the output as review.yaml
# 4. Validate the review file before posting
gh wheel review validate -f review.yaml --pr 42

# 5. Post the review to GitHub
gh wheel review post 42 -f review.yaml
```

## Requirements

- [GitHub CLI](https://cli.github.com/) ≥ 2.40.0
- `gh auth login` completed

## Installation

```bash
gh extension install UtakataKyosui/gh-wheel
```

## Update

```bash
gh extension upgrade wheel
```

To pin to a specific version:

```bash
gh extension upgrade wheel --version v0.3.0
```

---

## Claude Code Skills

The Claude Code Agent Skill for gh-wheel is published via `gh skill`.
Install it with a single command:

```bash
gh skill install UtakataKyosui/gh-wheel gh-wheel
```

This installs `gh-wheel/SKILL.md` plus per-command and workflow reference files
so Claude picks it up automatically in every session.

> **`gh wheel skill` is deprecated.** Future Claude Code skills for gh-wheel are
> published via `gh skill` and installed with the command above.

### What the skill teaches Claude

| Capability | Commands |
|---|---|
| Task management | `task`, `task today`, `task next`, `task close` |
| Code review | `review prompt`, `review validate`, `review post`, `review threads`, `review reply` |
| Dependency graph | `graph` |
| OKR metrics | `okr metrics` |
| Developer workflow | End-to-end Issue → PR → review-reply loop |
| Reviewer workflow | End-to-end review-request → AI review → post loop |

### Example prompts

```
/gh-wheel — list my open PRs and assigned Issues in this repo
/gh-wheel — plan today's work with a 4h budget
/gh-wheel — run an AI review on PR #42 and post it
```

---

## Commands

### `gh wheel task`

Browse and operate on PRs and Issues you are involved in as author or reviewer.

```bash
gh wheel task                    # open TUI (default)
gh wheel task -j                 # JSON output
gh wheel task --with-reviews     # include review status per PR
gh wheel task --with-issues      # include assigned Issues
gh wheel task --issues-only      # Issues only
gh wheel task -a                 # authored PRs only
gh wheel task -r                 # review-requested PRs only
gh wheel task -s closed          # filter by state: open | closed | all
```

| Flag | Default | Description |
|------|---------|-------------|
| `-s, --state` | `open` | Filter by state: `open`, `closed`, `all` |
| `-a, --author-only` | false | Show only PRs where you are the author |
| `-r, --review-only` | false | Show only PRs where review is requested from you |
| `-d, --include-drafts` | true | Include draft PRs |
| `-I, --with-issues` | false | Include Issues assigned to you |
| `--issues-only` | false | Show only Issues (implies `--with-issues`) |
| `--with-reviews` | false | Fetch review status for each PR |

#### Subcommands

**`gh wheel task close <N>`**

Close a PR or Issue by number. Prints the item's title and URL, then asks for confirmation. Pass `--json` to skip confirmation.

```bash
gh wheel task close 42
gh wheel task close 42 --json    # non-interactive, closes immediately
gh wheel task close 42 --dry-run # preview without closing
```

**`gh wheel task next [N]`**

Find an open, unassigned Issue that is **ready to start** and assign it to you. Useful for picking up the next piece of work in an Issue-Driven flow.

An Issue is "ready to start" when it is:

- open and **unassigned** (`no:assignee`),
- not labelled `epic` / `wontfix` / `invalid` / `duplicate` / `question`,
- (unless `--no-blockers`) **not already linked to a PR** and **not an aggregator with open sub-issues**.

Candidates are ranked, preferring `good first issue` (+2) and `help wanted` (+1) and de-prioritising `priority:low` (−2); ties break by ascending Issue number. By default the top candidate is shown and you re-enter its number to confirm before it is assigned.

```bash
gh wheel task next               # rank candidates, confirm, assign the top one
gh wheel task next --list        # only list candidates (no assignment)
gh wheel task next --dry-run     # preview the assignment without writing
gh wheel task next 16            # assign a specific Issue (#16)
gh wheel task next --json        # non-interactive: assign top candidate, emit JSON
gh wheel task next --no-blockers # skip blocker filtering (faster; no GraphQL)
```

| Flag | Default | Description |
|------|---------|-------------|
| `--list` | false | List candidates without assigning |
| `--yes` | false | Skip the confirmation prompt before assigning |
| `--no-blockers` | false | Skip blocker filtering (faster; no GraphQL queries) |
| `--limit` | `5` | Maximum number of candidates to show |
| `--label` | | Restrict candidates to a label |

JSON output uses `kind: "task_next_result"` (or `task_next_preview` for `--dry-run`) with a ranked `candidates` array and an `assigned` object (`null` when nothing was assigned).

**`gh wheel task today`**

Build a **read-only** plan of what to work on today, fitted to a time budget. It gathers everything on your plate, classifies each item, estimates per-category effort, and greedily fills the budget in priority order.

Items are collected and ranked by priority:

1. `review` — a PR awaiting **your** review (unblock others).
2. `pr-merge` — your **approved** PR, ready to merge.
3. `pr-fix` — your PR with **outstanding** changes requested (a PR you have already pushed a fix for is treated as awaiting re-review and skipped).
4. `in-progress` — an Issue **assigned to you** — continue it.
5. `new` — a fresh approachable Issue (same readiness rules as `task next`); only fetched when budget remains.

Each item gets a per-category effort estimate. The budget is filled greedily: items that fit go into `plan`, the rest into `deferred`. The single highest-priority item is **always** included even if it alone exceeds the budget (set `over_budget: true` in that case), so the plan is never empty when there is work. This command never assigns, labels, or merges anything, so `--dry-run` produces identical output.

```bash
gh wheel task today                 # plan within the default 6h budget
gh wheel task today --budget 4h     # smaller budget
gh wheel task today --json          # machine-readable plan
gh wheel task today --no-new        # skip fresh candidate Issues (triage only)
gh wheel task today --budget 90m --json
```

| Flag | Default | Description |
|------|---------|-------------|
| `--budget` | `6h` | Time budget for today (duration: `6h`, `90m`, `4h30m`) |
| `--review-effort` | `20` | Estimated minutes per PR review |
| `--merge-effort` | `10` | Estimated minutes to merge an approved PR |
| `--pr-effort` | `45` | Estimated minutes to address changes requested on your PR |
| `--issue-effort` | `90` | Estimated minutes per Issue (in-progress or new) |
| `--no-new` | false | Do not include fresh approachable Issues |
| `--no-blockers` | false | Skip blocker filtering for new Issues (faster; no GraphQL queries) |

JSON output uses `kind: "task_today_result"` with `plan` and `deferred` arrays, a `params` object (the budget and effort inputs), `total_estimated_minutes`, `over_budget`, and `truncated` (best-effort flag set when a search hit its 100-result cap).

**`gh wheel task schedule`**

Run a **self-contained background daemon** that periodically snapshots your task list to `<repo>/.git/my-tasks/current.json` — **no cron or launchd required**. Useful for keeping a fresh view of pending reviews/merges without re-querying GitHub on every command, and (with `--notify`) for a desktop nudge when work is waiting.

```bash
gh wheel task schedule add                 # watch the current repo every 5m, start the daemon
gh wheel task schedule add --interval 10m --notify   # custom interval + desktop notification
gh wheel task schedule add --review-only   # snapshot only PRs awaiting your review
gh wheel task schedule list                # registered repos + daemon status
gh wheel task schedule status              # daemon pid + total snapshots taken
gh wheel task schedule remove              # unregister the current repo (stops daemon if it was the last)
gh wheel task schedule start               # start the daemon manually
gh wheel task schedule stop                # stop the daemon (SIGTERM)
```

The daemon writes its config, pid, and log under `$XDG_CONFIG_HOME/gh-wheel/` (`schedules.json`, `daemon.pid`, `daemon.log`). Re-running `add` for a repo updates its settings while preserving run statistics.

| `add` flag | Default | Description |
|------|---------|-------------|
| `--interval` | `5m` | Snapshot interval (Go duration; minimum `1m`) |
| `-s, --state` | `open` | Filter by state: `open`, `closed`, `all` |
| `-a, --author-only` | false | Snapshot only PRs you authored |
| `-r, --review-only` | false | Snapshot only PRs where review is requested from you |
| `-d, --include-drafts` | true | Include draft PRs |
| `--with-reviews` | false | Fetch review status for each PR (slower) |
| `--notify` | false | Show a desktop notification after each snapshot (macOS) |
| `--no-start` | false | Register without starting the daemon |

JSON output uses `kind` values `schedule_add_result`, `schedule_list_result`, `schedule_remove_result`, `schedule_start_result`, `schedule_stop_result`, and `schedule_status_result`, each with an `entries` array and a `daemon` object (`{running, pid}`).


---

### `gh wheel graph`

Fetch and display dependency and reference graphs between Issues and PRs.

```bash
gh wheel graph                        # whole-repo graph (list format)
gh wheel graph --issue 42             # BFS from Issue #42
gh wheel graph --issue 42 --depth 3  # BFS depth 3
gh wheel graph --format tree          # tree view
gh wheel graph --format dot           # Graphviz DOT output
gh wheel graph -j                     # JSON output
gh wheel graph --label bug            # filter by label
gh wheel graph --milestone v1.0       # filter by milestone
```

| Flag | Default | Description |
|------|---------|-------------|
| `--issue` | `0` | BFS start issue number (0 = whole repo) |
| `--depth` | `2` | BFS depth from `--issue` |
| `--label` | | Filter nodes by label |
| `--milestone` | | Filter nodes by milestone title |
| `--no-timeline` | false | Skip cross-reference timeline queries |
| `--no-sub-issues` | false | Skip sub-issue queries |
| `--format` | `list` | Output format: `list` \| `tree` \| `dot` \| `json` |

---

### `gh wheel monitor`

Watch multiple GitHub repositories for new Issues and PRs in real time via a TUI dashboard.

```bash
gh wheel monitor
```

> Full multi-repo monitoring is under development.

---

### `gh wheel review`

Generate review prompts, validate AI review output, and post structured code reviews.

#### Subcommands

**`gh wheel review prompt <PR>`**

Fetch PR metadata and diff, then write a Markdown prompt suitable for AI review to stdout.

```bash
gh wheel review prompt 42          # print AI review prompt to stdout
gh wheel review prompt 42 | pbcopy # copy to clipboard
```

**`gh wheel review schema`**

Print the JSON Schema for the review output format to stdout. Use this to understand the structure expected by `review validate` and `review post`.

```bash
gh wheel review schema
```

**`gh wheel review validate -f <file>`**

Validate an AI-generated review JSON/YAML file before posting. Checks structure, required fields, and comment count thresholds.

```bash
gh wheel review validate -f review.yaml
gh wheel review validate -f review.json --pr 42 --strict
```

| Flag | Default | Description |
|------|---------|-------------|
| `-f, --file` | (required) | Path to the review JSON/YAML file |
| `--pr` | `0` | PR number (used for dynamic comment count threshold) |
| `--min-comments` | `0` | Override minimum comment count |
| `--strict` | false | Treat warnings as errors |
| `--format` | auto | File format: `yaml` \| `json` |
| `-R, --repo` | cwd | Repository (`owner/name`) |

**`gh wheel review post <PR> -f <file>`**

Validate and post an AI-generated structured review file to a GitHub Pull Request.

```bash
gh wheel review post 42 -f review.yaml
gh wheel review post 42 -f review.json --dry-run  # preview payload
gh wheel review post 42 -f review.yaml --strict
```

| Flag | Default | Description |
|------|---------|-------------|
| `-f, --file` | (required) | Path to the review JSON/YAML file |
| `--min-comments` | `0` | Minimum comment count (0 = dynamic default of 1) |
| `--strict` | false | Treat warnings as errors |
| `--dry-run` | false | Print payload JSON without posting |
| `--format` | auto | File format: `yaml` \| `json` |
| `-R, --repo` | cwd | Repository (`owner/name`) |

**`gh wheel review threads <PR>`**

List unresolved review threads for a pull request.

```bash
gh wheel review threads 42
gh wheel review threads 42 --json
```

**`gh wheel review reply <PR>`**

Post a reply to a specific pull request review comment.

```bash
gh wheel review reply 42 --comment-id 123456789 --body "Fixed in latest commit."
```

| Flag | Default | Description |
|------|---------|-------------|
| `--comment-id` | (required) | Reply target comment ID |
| `--body` | (required) | Reply text |
| `-R, --repo` | cwd | Repository (`owner/name`) |

---

### `gh wheel okr`

Compute GitHub activity metrics for OKR key-result tracking. Designed to feed
[okr-hub](https://github.com/UtakataKyosui/okr-hub)'s `okr-metrics-sync` skill.

**`gh wheel okr metrics --since <date> --until <date>`**

Aggregate your GitHub activity over a date range. By default the search is
**cross-repo** (`author:@me` / `reviewed-by:@me` across every repository you can
see), which suits personal OKRs. Pass `-R owner/repo` to scope to one repository.

```bash
# Cross-repo metrics for a period (does not require being inside a git repo)
gh wheel okr metrics --since 2026-04-01 --until 2026-09-30 --json

# Scope to a single repository
gh wheel okr metrics -R UtakataKyosui/gh-wheel --since 2026-04-01 --until 2026-09-30

# Match metrics onto key results (JSON shape produced by okr-hub's okr_parse.py)
gh wheel okr metrics --since 2026-04-01 --until 2026-09-30 \
  --krs '[{"label":"KR1","title":"PR品質向上","metrics_source":"github:avg_review_comments_per_pr"}]' \
  --json | jq '.kr_metrics'
```

| Flag | Default | Description |
|------|---------|-------------|
| `--since` | (required) | Start date `YYYY-MM-DD` (inclusive) |
| `--until` | (required) | End date `YYYY-MM-DD` (inclusive) |
| `--krs` | | Key results to match, as JSON: `[{"label","title","metrics_source"}]` |
| `-R, --repo` | all repos | Scope to a single repository (`owner/name`) |

The `metrics` object uses the same keys as okr-hub's `計測ソース: github:<key>`
fields, so output is consumable by the `okr-metrics-sync` skill:

`authored_prs_total`, `pr_count`, `merged_prs`, `avg_cycle_time_hours` (null if
nothing merged), `review_comments_received`, `avg_review_comments_per_pr`,
`reviewed_prs`, `issues_created`, `issues_closed`.

**Metric semantics** (read before using these in ratios):

- **Date axis differs per metric.** `pr_count` / `authored_prs_total`, `reviewed_prs`, `issues_created` are filtered by **creation** date; `merged_prs` by **merge** date; `issues_closed` by **close** date. Because `merged_prs` (merged-in-window) and `pr_count` (created-in-window) are different populations, a PR created before the window but merged inside it counts only in `merged_prs` — so `merged_prs / pr_count` can exceed 1. Treat each as an independent count, not a rate.
- **`reviewed_prs` is creation-dated.** It counts PRs *created* in the window that you reviewed (GitHub Search has no "reviewed-on" qualifier), not PRs you reviewed during the window.
- **`review_comments_received` / `avg_review_comments_per_pr` count conversation comments**, not inline code-review comments (the Search API's `comments` field). This matches okr-hub's `okr_github_metrics.py`.
- **Cross-repo scope.** Without `-R`, `author:@me` / `reviewed-by:@me` span **every repository you can access** (including org repos and forks), which may include work outside your "personal" OKRs. Use `-R owner/repo` to scope.
- Averages are computed over an enumerated sample capped at 1000 PRs; the counts (`*_total`, `pr_count`, …) stay exact via `total_count` beyond that cap.

#### Integrating with okr-hub

okr-hub currently computes these figures with
`plugins/okr-progress/scripts/okr_github_metrics.py`. To switch that skill to
gh-wheel (cross-repo, paginated, structured), have `okr-metrics-sync` call:

```bash
gh wheel okr metrics --since <period_start> --until <period_end> \
  --krs '<JSON array from okr_parse.py>' --json
```

and read `.metrics` and `.kr_metrics` from the output. Two behavioural notes
versus the Python script: metrics are **cross-repo by default** (pass `-R` for
single-repo parity), and an unauthenticated `gh` surfaces as **exit code 4 with
an error envelope** rather than an `{"available": false}` body — a successful run
always reports `"available": true`. (This wiring lives in the okr-hub repo and is
out of scope for gh-wheel itself.)

---

### `gh wheel describe`

Print gh-wheel's full command schema as machine-readable JSON. Includes the list of subcommands, their output kinds, and the complete exit code table — useful for AI agents discovering the CLI contract.

```bash
gh wheel describe
gh wheel describe | jq '.exit_codes'
```

---

### `gh wheel feedback`

Open an interactive TUI form to file a feature request or bug report against the gh-wheel repository on GitHub.

```bash
gh wheel feedback
```

---

### `gh wheel skill`

Generate a Claude Code Agent Skill (`SKILL.md`) that teaches an AI agent how to operate gh-wheel.

```bash
gh wheel skill                          # print SKILL.md to stdout
gh wheel skill -o ~/.claude/skills/     # write to directory
gh wheel skill --name my-wheel          # custom skill name
```

| Flag | Default | Description |
|------|---------|-------------|
| `--name` | `gh-wheel` | Skill name (frontmatter name and output directory) |
| `--description` | | Override the frontmatter description |
| `-o, --output` | stdout | Directory to write `<name>/SKILL.md` into |

---

## Global Flags

Available on all subcommands:

| Flag | Description |
|------|-------------|
| `-R, --repo` | Repository (`owner/repo`). Detected from cwd if omitted. |
| `-j, --json` | Output results as JSON |
| `--dry-run` | Validate input without sending API requests |
| `--jq <expr>` | Filter JSON output with a jq expression |
| `--no-report` | Do not offer to file an issue on unexpected errors |

## Exit Codes

| Code | Category | Meaning |
|------|----------|---------|
| 0 | success | Command completed successfully |
| 1 | general | Unexpected error |
| 2 | usage | Invalid arguments or flags |
| 3 | not_found | Requested resource not found |
| 4 | auth | Authentication error |
| 5 | validation | Input validation failed |
| 6 | api | GitHub API error |
