### レビュアー（Reviewer）ワークフロー

レビュー依頼された PR に対して AI 支援でレビューを行い、構造化されたレビューを投稿する。

```bash
# 1. レビュー依頼されている PR を一覧する
gh wheel task -r

# 2. AI レビュー用の Markdown プロンプトを生成する
gh wheel review prompt 42

# 3. AI にプロンプトを渡し、出力を review.yaml として保存する
# 4. 投稿前にレビューファイルを検証する
gh wheel review validate -f review.yaml --pr 42

# 5. レビューを GitHub に投稿する
gh wheel review post 42 -f review.yaml
```