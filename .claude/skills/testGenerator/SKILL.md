---
name: go-test-generator
description: Generates table-driven tests for Go functions
tools: Read, Write
---

テスト生成時のルール：

1. テーブルドリブンテスト形式を使用
2. テストケース名は日本語で明確に
3. 以下のケースを必ず含める：
   - 正常系
   - 境界値
   - エラーケース

テンプレート：
func TestXxx(t *testing.T) {
    tests := []struct {
        name    string
        input   InputType
        want    OutputType
        wantErr bool
    }{
        {"正常系: 基本的な入力", ...},
        {"異常系: nilの場合", ...},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ...
        })
    }
}