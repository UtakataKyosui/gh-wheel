# gh-wheel

A unified `gh` extension for Issue-Driven development, supporting two complementary workflows:

**As a developer** — Check an Issue, open a PR, respond to review comments, and report fixes back to reviewers.

**As a reviewer** — Pick up a review request, generate an AI-assisted review, and post structured comments to the PR.

```
gh wheel task     — browse and manage your PRs and Issues
gh wheel graph    — visualize Issue/PR dependency graphs
gh wheel monitor  — watch multiple repos in a live TUI
gh wheel review   — AI-assisted code review workflows
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
gh wheel task prompt 42

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

**`gh wheel task prompt <PR>`**

Fetch PR metadata and diff, then write a Markdown prompt suitable for AI review to stdout.

```bash
gh wheel task prompt 42          # print AI review prompt to stdout
gh wheel task prompt 42 | pbcopy # copy to clipboard
```

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
