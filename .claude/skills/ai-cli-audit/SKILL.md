---
name: ai-cli-audit
description: gh-wheel の AIエージェント対応CLI設計（8原則）への準拠状況を監査し、不足している実装を報告・修正する
---

# AI Agent CLI 準拠監査スキル

gh-wheel を「AIエージェント対応CLI設計の8原則」（https://zenn.dev/assign/articles/b3d1d07d385b87）に照らして監査する。

## 監査チェックリスト

### 原則1: 構造化出力（schema_version + kind エンベロープ）

各 JSON 出力コマンドを確認する:

```bash
# JSON 出力する全コマンドのファイルを調査
grep -rE "jsonout.Print|json.MarshalIndent|json.Marshal" cmd/ --include="*.go" -l
```

確認項目:
- [ ] `schema_version` フィールドが struct に定義されているか
- [ ] `kind` フィールドが struct に定義されているか
- [ ] fetch/build 時に `"v1"` と固有の `kind` 文字列が設定されているか
- [ ] `cmd/describe/describe.go` の `spec.Commands` にコマンドが登録されているか

### 原則2: セマンティック終了コード

```bash
grep -rE "fmt.Errorf|errors.New" cmd/ --include="*.go" | grep -v "_test.go"
```

確認項目:
- [ ] `fmt.Errorf` や `errors.New` が直接 return されていないか（`cliexit.New*` でラップされているか）
- [ ] 「見つからない」エラーに `CodeNotFound = 3` が使われているか
- [ ] 認証エラーに `CodeAuth = 4` が使われているか
- [ ] バリデーションエラーに `CodeValidation = 5` が使われているか

### 原則3: 非対話モード

確認項目:
- [ ] 確認プロンプト（`bufio.Scanner`, `fmt.Scan` 等）が `--force` なしで実行を要求していないか
- [ ] TUI コマンドに `--dry-run` フラグがあるか、または非対話環境でスキップされるか

### 原則4: Noun-Verb 文法

```bash
grep -r "cobra.Command" cmd/ --include="*.go" -A2 | grep "Use:"
```

確認項目:
- [ ] サブコマンドの `Use` がリソース名（名詞）または `resource verb` 形式か
- [ ] ハイフン結合の複合動詞（`list-tasks` 等）が使われていないか

### 原則5: スキーマ自己記述

```bash
grep -A2 "CommandEntry{" cmd/describe/describe.go
```

確認項目:
- [ ] 全サブコマンドが `spec.Commands` リストに登録されているか
- [ ] `OutputKind` が設定されているか（JSON 出力するコマンドのみ）
- [ ] 全終了コードが `spec.ExitCodes` に記載されているか

### 原則6: アクション可能なエラー

```bash
grep -rE "NextStep|next_step" internal/ cmd/ --include="*.go" | grep -v "_test.go"
```

確認項目:
- [ ] 認証エラーに `NextStep` が設定されているか
- [ ] バイナリ未インストールエラーに `NextStep` が設定されているか
- [ ] その他の回復可能なエラーに `NextStep` が設定されているか

### 原則7: `--dry-run` サポート

```bash
grep -rE "dry-run|DryRun|dryRun" cmd/ --include="*.go" | grep -v "_test.go"
```

確認項目:
- [ ] 書き込み操作コマンドが `--dry-run` フラグを参照しているか

### 原則8: コンポーザビリティ

確認項目:
- [ ] 配列フィールドが `nil` ではなく空スライスで初期化されているか（`json:"..., omitempty"` を使わない）
- [ ] `--jq` フラグが `jsonout.Print(result, jqExpr)` で適用されているか

## 実行手順

1. 各チェックリストの項目を上記コマンドで確認する
2. 違反項目を列挙する
3. 修正が必要な項目をユーザーに報告し、承認を得てから修正する

## 修正優先度

| 優先度 | 原則 | 理由 |
|--------|------|------|
| 高 | 1, 2, 6 | エージェントが JSON を解析できない / 終了コードで判断できない |
| 中 | 5, 3 | セルフドキュメントなしでは新コマンド追加のたびに人間の介入が必要 |
| 低 | 4, 7, 8 | 利便性向上だが必須ではない |
