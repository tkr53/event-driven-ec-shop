---
name: conventional-commits
description: Generates commit messages in Conventional Commits format
tools: Bash
---

git diff --staged を確認し、以下の形式でメッセージを生成：

<type>(<scope>): <subject>

type: feat, fix, refactor, test, docs, chore
scope: 変更対象のモジュール名
subject: 日本語で50文字以内

例: feat(order): 注文キャンセル機能を追加