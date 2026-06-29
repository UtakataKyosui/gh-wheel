### 開発者（Developer）ワークフロー

Issue の内容を確認して PR を作成し、レビューコメントに対応して修正内容を返信で報告する。

```bash
# 1. 自分が関わる PR・Issue を一覧する
gh wheel task

# 2. Issue の依存関係を可視化する
gh wheel graph --issue 42

# 3. PR の未解決レビュースレッドを確認する
gh wheel review threads 42

# 4. レビューコメントへ修正完了を返信する
gh wheel review reply 42 --comment-id 123456789 --body "Fixed in latest commit."
```
