### `gh wheel review`

Code review workflows for GitHub PRs

Generate review prompts, validate AI review output, and post structured code reviews.

#### `gh wheel review post`

Post a structured review (JSON/YAML) to a GitHub PR

Validate and post an AI-generated structured review file to a GitHub Pull Request.

```
gh wheel review post <PR> -f <file> [flags]
```

フラグ:

- `--dry-run` — Print payload JSON without posting to GitHub
- `-f, --file` — Path to the review JSON/YAML file (required)
- `--format` — File format: yaml|json (default: auto-detect by extension)
- `--min-comments` — Minimum comment count (0 = use default of 1) (default "0")
- `--repo` — Repository (owner/name), defaults to current directory
- `--strict` — Treat warnings as errors

#### `gh wheel review prompt`

Output a Markdown review prompt for a PR to stdout

Fetch PR metadata and diff, then write a Markdown prompt suitable
for AI review to stdout.

Example:
  gh wheel review prompt 123 | claude --print > review.json

```
gh wheel review prompt <PR>
```

#### `gh wheel review reply`

Post a reply to a PR review comment

Post a reply to a specific pull request review comment by comment ID.

```
gh wheel review reply <PR> [flags]
```

フラグ:

- `--body` — reply text (required)
- `--comment-id` — reply target comment ID (required)
- `-R, --repo` — repository (owner/name); defaults to cwd

#### `gh wheel review schema`

Print the JSON Schema for review output to stdout

```
gh wheel review schema
```

#### `gh wheel review threads`

List unresolved review threads for a pull request

Fetches all review threads for the given PR and prints the ones that are neither resolved nor outdated.

```
gh wheel review threads <PR> [flags]
```

フラグ:

- `--json` — Output as JSON
- `-R, --repo` — Repository (OWNER/REPO). Defaults to current directory's repo.

#### `gh wheel review validate`

Validate an AI-generated review JSON/YAML file before posting

Gate-keeper that validates AI-generated review JSON/YAML before posting to GitHub.

```
gh wheel review validate -f <file> [flags]
```

フラグ:

- `-f, --file` — Path to the review JSON/YAML file (required)
- `--format` — File format: yaml|json (default: auto-detect by extension)
- `--min-comments` — Override minimum comment count (0 = use dynamic) (default "0")
- `--pr` — PR number (used to fetch changed_files for dynamic threshold) (default "0")
- `--repo` — Repository (owner/name), defaults to current directory
- `--strict` — Treat warnings as errors
