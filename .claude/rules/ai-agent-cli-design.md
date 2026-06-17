# AIエージェント対応CLI設計の8原則

出典: https://zenn.dev/assign/articles/b3d1d07d385b87

gh-wheel の CLI 設計・実装を変更するときは、以下の原則を遵守する。

---

## 必須の4原則

### 1. 構造化出力（schema_version + kind エンベロープ）

JSON 出力には必ず `schema_version` と `kind` を含む。

```json
{
  "schema_version": "v1",
  "kind": "task_result",
  "repository": "owner/repo",
  "prs": [...]
}
```

**How to apply:**
- `jsonout.Print(result, jqExpr)` で出力する前に、struct に `SchemaVersion string \`json:"schema_version"\`` と `Kind string \`json:"kind"\`` を定義する
- `SchemaVersion = "v1"` で固定、`Kind` はコマンドごとに一意の snake_case 名を付ける（例: `task_result`, `graph_result`, `command_schema`）
- 新しいサブコマンドを追加するときは必ず JSON エンベロープを実装する

### 2. セマンティック終了コード（5カテゴリ以上）

`cliexit` パッケージの定数を使い、エラーカテゴリを正確に反映した終了コードを返す。

| コード | カテゴリ | 定数 |
|--------|----------|------|
| 0 | success | `CodeSuccess` |
| 1 | general | `CodeGeneral` |
| 2 | usage | `CodeUsage` |
| 3 | not_found | `CodeNotFound` |
| 4 | auth | `CodeAuth` |
| 5 | validation | `CodeValidation` |
| 6 | api | `CodeAPI` |

**How to apply:**
- 「見つからない」系は `cliexit.NewNotFound(ErrCodeNotFound, err)` を使う（exit 1 にしない）
- 認証エラーには必ず `cliexit.NewAuth(...)` を使う
- API エラーには `cliexit.NewAPI(...)` を使う
- `fmt.Errorf` をそのまま return しない（`NewGeneral` でラップする）

### 3. 非対話モードとシークレット管理

エージェントが確認プロンプトなしで実行できるように設計する。

**How to apply:**
- 破壊的操作には `--force` フラグを用意し、非対話環境でも実行できるようにする
- 認証情報は環境変数か stdin 経由で受け取る（引数に直接含めない）
- `gh wheel feedback` のような TUI コマンドは `--dry-run` を実装し、非対話でも動作確認できるようにする

### 4. Noun-Verb 文法

サブコマンドは `resource action` 形式で階層化する。

```bash
gh wheel task list       # ○ (task が noun、list が verb)
gh wheel review post     # ○
gh wheel task-list       # ✗ ハイフン結合は禁止
```

**How to apply:**
- 新しいサブコマンドは単一の名詞（リソース名）とし、操作は子コマンドとして追加する
- 動詞が不要な場合（`graph`, `describe` など）は単体のコマンドとして定義する

---

## 強く推奨される2原則

### 5. スキーマ自己記述（`describe` サブコマンド）

`gh wheel describe` でコマンドスキーマを JSON 出力する（実装済み）。

**How to apply:**
- 新しいサブコマンドを追加したら `cmd/describe/describe.go` の `spec.Commands` リストに追記する
- 出力する `kind` 名と `OutputKind` を一致させる
- 終了コードを変更したら `spec.ExitCodes` も更新する

### 6. アクション可能なエラー

エラーに「次に何をすべきか」を含める。

```json
{
  "error": {
    "category": "auth",
    "exit_code": 4,
    "code": "AUTH_NOT_LOGGED_IN",
    "message": "not authenticated for github.com",
    "next_step": "Run: gh auth login"
  }
}
```

**How to apply:**
- `cliexit.Error.NextStep` フィールドに具体的な操作コマンドを設定する
- 認証エラー: `e.NextStep = "Run: gh auth login"`
- バイナリ未インストール: `e.NextStep = "Install the GitHub CLI: https://cli.github.com"`
- 新しいエラーを追加するとき、エージェントが自動回復できる操作があれば `NextStep` に記載する

---

## 推奨される2原則

### 7. 冪等操作と `--dry-run`

`--dry-run` フラグは root persistent flag として実装済み。サブコマンドで活用する。

**How to apply:**
- 書き込み操作を行うコマンドでは `--dry-run` フラグを確認して副作用をスキップする
- dry-run 時も JSON エンベロープつきのプレビュー結果を stdout に出力する

### 8. コンポーザビリティ（jq + パイプ）

`--jq` フラグは root persistent flag として実装済み。`jsonout.Print(result, jqExpr)` で自動適用される。

**How to apply:**
- JSON 出力時に `jqExpr` パラメータを `jsonout.Print` / `jsonout.Write` に渡す
- 配列フィールドは nil ではなく空スライスで初期化する（`null` ではなく `[]` で出力）
