---
name: gh-wheel
description: 'A unified gh extension for Issue-Driven development。以下の場合に使用: (1) Issue を確認して PR を作成・管理したいとき (2) レビュー依頼された PR に対してレビューを行い投稿したいとき (3) レビューコメントに対応して返信で報告したいとき (4) Issue・PR の依存グラフを確認したいとき'
---

# gh-wheel

gh-wheel integrates task management, issue relationship graphs,
and code review workflows into a single gh extension.

このスキルは `gh wheel` CLI を操作するためのリファレンスです。
以下のコマンドを実行して gh-wheel を操作してください。

## ワークフロー

gh-wheel は2つのロールのワークフローを支援します。

- [開発者（Developer）ワークフロー](./reference/developer-workflow.md)

- [レビュアー（Reviewer）ワークフロー](./reference/reviewer-workflow.md)

## コマンドリファレンス

- [describe](./commands/describe.md) — Print gh-wheel's command schema as JSON
- [feedback](./commands/feedback.md) — Submit a feature request or bug report for gh-wheel
- [graph](./commands/graph.md) — Visualize GitHub Issue/PR relationship graphs
- [okr](./commands/okr.md) — Compute GitHub activity metrics for OKR key results
- [review](./commands/review.md) — Code review workflows for GitHub PRs
- [task](./commands/task.md) — Manage your GitHub tasks (PRs and Issues)

## グローバルフラグ

すべてのサブコマンドで利用できます。

- `--dry-run` — Validate input without sending API requests
- `--jq` — Filter JSON output with a jq expression
- `-j, --json` — Output results as JSON
- `--no-report` — Do not offer to file an issue when an unexpected error occurs
- `-R, --repo` — Repository in owner/repo format (detected from cwd if omitted)

## 補足

- `--json` を付けると機械可読な JSON を stdout に出力します。スクリプトや AI 連携ではこちらを使ってください。
- `--jq <式>` で JSON 出力を絞り込めます。
- `--repo owner/repo` で対象リポジトリを明示できます（省略時は cwd から検出）。
- 予期しないエラーや panic が発生すると、対話実行時に gh-wheel への Issue 起票（auto-report）が提案されます。`--no-report` または環境変数 `GH_WHEEL_NO_REPORT` で抑止できます。
