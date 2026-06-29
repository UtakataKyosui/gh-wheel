### `gh wheel describe`

Print gh-wheel's command schema as JSON

Print gh-wheel's full command schema as machine-readable JSON.

The output includes the list of subcommands, their output kinds, and
the complete exit code table with categories — making it easy for AI
agents and scripts to understand the CLI contract without parsing --help.

```
gh wheel describe
```