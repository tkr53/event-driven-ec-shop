---
name: team-code-review
description: Reviews code changes following our team's coding standards
tools: Bash, Read
---

レビュー時は以下をチェック：

## Go言語の規約
- エラーハンドリング: errは必ずチェック、wrapして返す
- 命名: CamelCase、略語は避ける
- コメント: 公開関数には必ずGoDoc形式で記述

## アーキテクチャ
- レイヤー違反がないか（handler → usecase → repository）
- 依存の方向が正しいか

## テスト
- テーブルドリブンテストを使用しているか
- エッジケースをカバーしているか