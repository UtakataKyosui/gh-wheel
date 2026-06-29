### `gh wheel graph`

Visualize GitHub Issue/PR relationship graphs

Fetch and display dependency and reference graphs between Issues and PRs.

Examples:
  # Show the full repository graph as a list
  gh wheel graph

  # Show a BFS subgraph rooted at issue #10, depth 3
  gh wheel graph --issue 10 --depth 3

  # Export graph as DOT for Graphviz
  gh wheel graph --format dot

  # Export graph as JSON
  gh wheel graph --format json

  # Filter by label
  gh wheel graph --label "bug"

```
gh wheel graph [flags]
```

フラグ:

- `--depth` — BFS depth from --issue (ignored when --issue=0) (default "2")
- `--format` — Output format: list|tree|dot|json (default "list")
- `--issue` — BFS start issue number (0 = fetch whole repo) (default "0")
- `--jq` — jq filter applied to JSON output
- `--label` — Filter nodes by label
- `--milestone` — Filter nodes by milestone title
- `--no-sub-issues` — Skip sub-issue queries
- `--no-timeline` — Skip cross-reference timeline queries