# ユニットテスト実装計画

## 概要

Event-Driven ECショップのユニットテストを実装します。現在、39のGoファイルに対してテストは0件です。

## 1. セットアップ

### 依存関係の追加

```bash
go get github.com/stretchr/testify
go get go.uber.org/mock/mockgen
```

### Makefile に追加するターゲット

```makefile
test:
	go test -v ./...

test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

test-race:
	go test -v -race ./...

mocks:
	mockgen -source=internal/infrastructure/store/interface.go \
		-destination=internal/infrastructure/store/mocks/event_store_mock.go -package=mocks
	mockgen -source=internal/infrastructure/store/read_store_interface.go \
		-destination=internal/infrastructure/store/mocks/read_store_mock.go -package=mocks
```

## 2. 作成するファイル

### モック（手動実装）

| ファイル | 用途 |
|---------|------|
| `internal/infrastructure/store/mocks/event_store_mock.go` | EventStoreInterface のモック |
| `internal/infrastructure/store/mocks/read_store_mock.go` | ReadStoreInterface のモック |

### テストファイル（優先度順）

| ファイル | カバレッジ目標 | 優先度 |
|---------|--------------|--------|
| `internal/auth/password_test.go` | 95%+ | HIGH |
| `internal/auth/jwt_test.go` | 95%+ | HIGH |
| `internal/domain/order/aggregate_test.go` | 100% | CRITICAL |
| `internal/domain/user/aggregate_test.go` | 90%+ | HIGH |
| `internal/domain/product/aggregate_test.go` | 90%+ | HIGH |
| `internal/domain/inventory/aggregate_test.go` | 90%+ | HIGH |
| `internal/domain/cart/aggregate_test.go` | 85%+ | MEDIUM |
| `internal/domain/category/aggregate_test.go` | 85%+ | MEDIUM |
| `internal/command/handler_test.go` | 95%+ | CRITICAL |
| `internal/query/handler_test.go` | 85%+ | MEDIUM |
| `internal/projection/projector_test.go` | 90%+ | HIGH |
| `internal/api/middleware/auth_test.go` | 85%+ | MEDIUM |

## 3. 主要なテストケース

### 3.1 auth（セキュリティ重要）

**password_test.go:**
- `TestHashPassword_ValidPassword` - 8文字以上でハッシュ成功
- `TestHashPassword_ShortPassword` - 8文字未満で `ErrPasswordTooShort`
- `TestCheckPassword_CorrectPassword` - 正しいパスワードで `true`
- `TestCheckPassword_WrongPassword` - 間違いで `false`

**jwt_test.go:**
- `TestJWTService_GenerateAccessToken_Success`
- `TestJWTService_ValidateAccessToken_Valid` - クレーム検証
- `TestJWTService_ValidateAccessToken_Expired` - `ErrExpiredToken`
- `TestJWTService_ValidateAccessToken_Invalid` - `ErrInvalidToken`
- `TestJWTService_ValidateAccessToken_WrongSignature`

### 3.2 order（状態遷移 100%カバレッジ必須）

```
状態遷移図:
pending -> paid -> shipped
pending -> cancelled
paid -> cancelled
shipped -> (キャンセル不可)
```

- `TestService_Place_Success` - 注文作成、`OrderPlaced`イベント
- `TestService_Place_EmptyItems` - `ErrEmptyOrder`
- `TestService_Pay_FromPending` - pending→paid 成功
- `TestService_Pay_AlreadyPaid` - `ErrOrderAlreadyPaid`
- `TestService_Pay_Cancelled` - `ErrOrderCancelled`
- `TestService_Ship_FromPaid` - paid→shipped 成功
- `TestService_Ship_FromPending` - `ErrOrderNotPaid`
- `TestService_Cancel_FromShipped` - `ErrOrderShipped`

### 3.3 command（補償トランザクション重要）

- `TestHandler_PlaceOrder_Success` - 注文→在庫予約→カートクリア
- `TestHandler_PlaceOrder_EmptyCart` - `ErrEmptyOrder`
- `TestHandler_PlaceOrder_InsufficientStock` - `ErrInsufficientStock`
- `TestHandler_PlaceOrder_CompensatingTransaction` - 在庫予約失敗時のロールバック検証

### 3.4 その他のドメイン

**user:**
- `TestIsValidEmail_ValidEmails` / `InvalidEmails`
- `TestService_Register_Success` / `InvalidEmail` / `EmptyName`
- `TestService_ChangePassword_Success` / `ShortPassword`

**product:**
- `TestService_Create_ValidProduct` / `EmptyName` / `ZeroPrice`
- `TestService_Update_Success` / `NotFound`

**inventory:**
- `TestService_AddStock_ValidQuantity` / `ZeroQuantity`
- `TestService_Reserve_ValidQuantity`
- `TestInventory_AvailableStock` - 計算検証

**cart:**
- `TestService_AddItem_Success` / `EmptyProductID` / `ZeroQuantity`
- `TestGetCartID` - "cart-" + userID 形式

**category:**
- `TestService_Create_ValidCategory` / `EmptyName`
- `TestGenerateSlug_Various` - スラッグ生成検証

### 3.5 projection

- `TestProjector_HandleOrderPlaced` - read_orders に保存
- `TestProjector_HandleOrderPaid` - ステータス更新
- `TestProjector_HandleStockReserved` - 在庫計算更新
- `TestProjector_CalculateCartTotal` - 合計計算検証

## 4. 実装順序

### Phase 1: 基盤（1-2日）
1. テスト依存関係を `go.mod` に追加
2. `mocks/` ディレクトリにモック実装を作成
3. Makefile にテストターゲット追加
4. `internal/auth/` のテスト実装

### Phase 2: ドメイン層（2-3日）
5. `internal/domain/order/` テスト（状態遷移 100%）
6. `internal/domain/user/` テスト
7. `internal/domain/product/` テスト
8. `internal/domain/inventory/` テスト
9. `internal/domain/cart/` テスト
10. `internal/domain/category/` テスト

### Phase 3: アプリケーション層（2日）
11. `internal/command/` テスト（補償トランザクション含む）
12. `internal/query/` テスト
13. `internal/projection/` テスト

### Phase 4: インフラ層（1日）
14. `internal/api/middleware/` テスト

## 5. 検証方法

```bash
# 全テスト実行
make test

# カバレッジレポート生成
make test-coverage
# coverage.html をブラウザで確認

# レースコンディション検出
make test-race

# 特定パッケージのテスト
go test -v -cover ./internal/auth/...
go test -v -cover ./internal/domain/order/...
go test -v -cover ./internal/command/...
```

**目標カバレッジ: 全体 85%+**

## 6. 重要ファイル

- `internal/infrastructure/store/interface.go` - EventStoreInterface 定義
- `internal/infrastructure/store/read_store_interface.go` - ReadStoreInterface 定義
- `internal/domain/order/aggregate.go` - 注文状態遷移ロジック
- `internal/command/handler.go` - 補償トランザクションロジック
