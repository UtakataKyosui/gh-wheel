# gh-wheel

A unified `gh` extension for Issue-Driven development.

```
gh wheel task     — browse and manage your PRs and Issues
gh wheel graph    — visualize Issue/PR dependency graphs
gh wheel monitor  — watch multiple repos in a live TUI
gh wheel review   — AI-assisted code review workflows
gh wheel describe — print command schema as JSON (for AI agents)
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
