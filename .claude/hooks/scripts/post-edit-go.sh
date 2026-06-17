#!/bin/bash
# PostToolUse hook: Edit/Write 後に gofmt + go vet を実行する（Go ファイルのみ）

input=$(cat)
file_path=$(echo "$input" | jq -r '.tool_input.file_path // ""')

[ -n "$file_path" ] || exit 0
[ -f "$file_path" ] || exit 0

# Go ファイル以外はスキップ
[[ "$file_path" == *.go ]] || exit 0

# gofmt
gofmt -w "$file_path"

# go vet（リポジトリルートから実行）
repo_root=$(git -C "$(dirname "$file_path")" rev-parse --show-toplevel 2>/dev/null) || exit 0
cd "$repo_root" && go vet ./... 2>&1 | head -20 >&2 || true
