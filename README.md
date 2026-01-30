# EC Shop - Event Driven + CQRS Learning Project

Go + AWS Kinesis Data Streams + DynamoDB + PostgreSQL を使った**イベント駆動アーキテクチャ**と**CQRS（Command Query Responsibility Segregation）パターン**を学ぶためのECサイトバックエンドです。

**特徴:**
- **書き込みDB**と**読み取りDB**を完全分離したCQRS実装
- **DynamoDB → Kinesis 自動CDC**によるイベント駆動の読み取りモデル更新
- **Lambda**による非同期イベント処理
- **イベントソーシング**による完全な履歴管理

## 目次

- [アーキテクチャ概要](#アーキテクチャ概要)
- [CQRS パターンとは](#cqrs-パターンとは)
- [イベント駆動アーキテクチャとは](#イベント駆動アーキテクチャとは)
- [プロジェクト構成](#プロジェクト構成)
- [技術スタック](#技術スタック)
- [セットアップ](#セットアップ)
- [使い方](#使い方)
- [API リファレンス](#api-リファレンス)
- [ドメインモデル](#ドメインモデル)
- [イベント一覧](#イベント一覧)
- [データフロー](#データフロー)
- [コード解説](#コード解説)
- [学習ポイント](#学習ポイント)

---

## アーキテクチャ概要

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Client (Browser)                                │
│                              http://localhost:3000                           │
└─────────────────────────────────┬───────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              API Server (:8080)                              │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                         HTTP Router                                    │  │
│  │   POST /products, POST /cart/items, POST /orders ... (Commands)       │  │
│  │   GET /products, GET /cart, GET /orders ...          (Queries)        │  │
│  └───────────────────────────────┬───────────────────────────────────────┘  │
│                                  │                                           │
│          ┌───────────────────────┴───────────────────────┐                  │
│          ▼                                               ▼                  │
│  ┌───────────────────┐                         ┌───────────────────┐       │
│  │  Command Handler  │                         │  Query Handler    │       │
│  │  (Write側)        │                         │  (Read側)         │       │
│  └─────────┬─────────┘                         └─────────┬─────────┘       │
│            │                                             │                  │
│            ▼                                             ▼                  │
│  ┌───────────────────┐                         ┌───────────────────┐       │
│  │  Domain Services  │                         │  PostgreSQL       │       │
│  │  (Product, Cart,  │                         │  (Read Tables)    │       │
│  │   Order, Inventory)│                         │  read_products    │       │
│  └─────────┬─────────┘                         │  read_carts       │       │
│            │                                   │  read_orders      │       │
│            ▼                                   │  read_inventory   │       │
│  ┌───────────────────┐                         └───────────────────┘       │
│  │  Event Store      │                                   ▲                  │
│  │  (DynamoDB)       │                                   │                  │
│  │  events table     │                                   │                  │
│  └─────────┬─────────┘                                   │                  │
│            │                                             │                  │
└────────────┼─────────────────────────────────────────────┼──────────────────┘
             │                                             │
             ▼ (自動 CDC)                                  │
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Kinesis Data Streams                                 │
│                       Stream: ec-events                                      │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  ProductCreated → StockAdded → ItemAddedToCart → OrderPlaced → ...  │    │
│  └────────────────────────────────────┬────────────────────────────────┘    │
└───────────────────────────────────────┼─────────────────────────────────────┘
                              ┌─────────┴─────────┐
                              ▼                   ▼
┌─────────────────────────────────────┐  ┌─────────────────────────────────────┐
│      Lambda Projector               │  │      Lambda Notifier                │
│  ┌───────────────────────────────┐  │  │  ┌───────────────────────────────┐  │
│  │ Kinesis → Event Handler       │  │  │  │ Kinesis → Event Handler       │  │
│  │        → PostgreSQL           │  │  │  │        → SMTP (Mailpit)       │  │
│  │                               │  │  │  │                               │  │
│  │ ProductCreated                │  │  │  │ OrderPlaced                   │  │
│  │   → read_products に INSERT   │  │  │  │   → 注文確認メール送信         │  │
│  │ StockAdded                    │  │  │  └───────────────────────────────┘  │
│  │   → read_inventory に UPDATE  │  │  └─────────────────────────────────────┘
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
```

### DB構成（CQRS分離）

| 用途 | ストレージ | 説明 |
|------|----------|------|
| **Write DB** | DynamoDB `events` | イベントストア（追記専用） |
| **Write DB** | DynamoDB `snapshots` | スナップショット |
| **Read DB** | PostgreSQL `read_products` | 商品一覧クエリ用 |
| **Read DB** | PostgreSQL `read_carts` | カート情報クエリ用 |
| **Read DB** | PostgreSQL `read_orders` | 注文履歴クエリ用 |
| **Read DB** | PostgreSQL `read_inventory` | 在庫情報クエリ用 |

### DynamoDB → Kinesis 自動CDC

従来の Kafka 構成では、アプリケーション側で DynamoDB 書き込み後に明示的に Kafka へ Publish する必要がありました。この設計には「DynamoDB 書き込み成功 → Kafka Publish 失敗」でデータ不整合が発生するリスクがありました。

現在の構成では、**DynamoDB Kinesis 統合**を使用することで：
- DynamoDB への書き込みが完了すると**自動的に Kinesis へストリーミング**
- アプリケーション側でのイベント発行処理が不要
- **データ不整合のリスクを解消**

---

## CQRS パターンとは

**CQRS (Command Query Responsibility Segregation)** は、データの「書き込み（Command）」と「読み取り（Query）」を分離するアーキテクチャパターンです。

### 従来のCRUDアーキテクチャ

```
┌─────────┐     ┌─────────┐     ┌─────────┐
│ Client  │────▶│  API    │────▶│   DB    │
└─────────┘     └─────────┘     └─────────┘
                    │                │
                    │  同じモデルで   │
                    │  読み書き両方   │
                    └────────────────┘
```

### CQRSアーキテクチャ（本プロジェクトの実装）

```
                    ┌─────────────────────┐
                    │      Command        │      ┌──────────────────┐
          Write ───▶│  (データ変更)        │─────▶│ DynamoDB         │
                    └─────────────────────┘      │ events テーブル   │
                                                 └────────┬─────────┘
                                                          │ (自動CDC)
                                                          ▼
                                                 ┌──────────────────┐
                                                 │ Kinesis Streams  │
                                                 │  (自動配信)       │
                                                 └────────┬─────────┘
                                                          │
                                                          ▼
                                                 ┌──────────────────┐
                                                 │ Lambda Projector │
                                                 │ (イベント変換)    │
                                                 └────────┬─────────┘
                    ┌─────────────────────┐               │
                    │       Query         │      ┌────────▼─────────┐
          Read ◀────│  (データ取得)        │◀─────│ PostgreSQL       │
                    └─────────────────────┘      │ read_* テーブル   │
                                                 └──────────────────┘
```

### CQRSのメリット

| メリット | 説明 |
|---------|------|
| **スケーラビリティ** | 読み取りと書き込みを独立してスケールできる |
| **最適化** | 読み取り用に最適化されたデータモデルを使用できる |
| **複雑なドメイン** | 複雑なビジネスロジックを書き込み側に集中できる |
| **パフォーマンス** | 読み取り専用のキャッシュやビューを活用できる |

### CQRSのデメリット

| デメリット | 説明 |
|-----------|------|
| **複雑性** | システム全体が複雑になる |
| **結果整合性** | 読み取りデータが最新でない可能性がある |
| **開発コスト** | 実装・運用コストが増加する |

---

## イベント駆動アーキテクチャとは

**イベント駆動アーキテクチャ (Event-Driven Architecture)** は、システム内の状態変更を「イベント」として表現し、そのイベントを介してコンポーネント間が連携するアーキテクチャです。

### イベントソーシング

このプロジェクトでは**イベントソーシング**パターンを採用しています。

```
従来: 現在の状態のみを保存
┌─────────────────────────────────┐
│ products table                  │
│ id: 1, name: "Tシャツ", stock: 50 │  ← 現在の状態のみ
└─────────────────────────────────┘

イベントソーシング: すべての変更履歴を保存
┌─────────────────────────────────────────────────────────────────┐
│ events table                                                     │
│ 1. ProductCreated { id: 1, name: "Tシャツ", stock: 100 }          │
│ 2. StockReserved { product_id: 1, quantity: 30 }                 │
│ 3. StockReserved { product_id: 1, quantity: 20 }                 │
│ → 現在の状態: stock = 100 - 30 - 20 = 50                          │
└─────────────────────────────────────────────────────────────────┘
```

### イベントソーシングのメリット

| メリット | 説明 |
|---------|------|
| **完全な履歴** | すべての変更履歴が残る（監査証跡） |
| **時間遡行** | 過去の任意の時点の状態を再現できる |
| **デバッグ** | 何が起きたかを正確に追跡できる |
| **イベント再生** | イベントを再生して新しいビューを構築できる |

---

## プロジェクト構成

```
event-driven-app/
├── cmd/                          # エントリーポイント
│   ├── api/
│   │   └── main.go              # APIサーバーのメイン
│   └── lambda/                  # Lambda関数
│       ├── projector/
│       │   └── main.go          # Lambda Projector（Read DB更新）
│       └── notifier/
│           └── main.go          # Lambda Notifier（メール送信）
│
├── internal/                     # 内部パッケージ
│   ├── api/                     # HTTP API層
│   │   ├── handlers.go          # HTTPハンドラー
│   │   └── router.go            # ルーティング設定
│   │
│   ├── command/                 # コマンド層（CQRS の Command側）
│   │   ├── commands.go          # コマンド定義（DTO）
│   │   └── handler.go           # コマンドハンドラー
│   │
│   ├── query/                   # クエリ層（CQRS の Query側）
│   │   ├── handler.go           # クエリハンドラー
│   │   └── read_models.go       # 読み取りモデル型エイリアス
│   │
│   ├── readmodel/               # 読み取りモデル定義（共通）
│   │   └── models.go            # ProductReadModel, CartReadModel, etc.
│   │
│   ├── domain/                  # ドメイン層（ビジネスロジックの中核）
│   │   ├── product/
│   │   │   ├── aggregate.go     # 商品集約
│   │   │   └── events.go        # 商品ドメインイベント
│   │   ├── cart/
│   │   │   ├── aggregate.go     # カート集約
│   │   │   └── events.go        # カートドメインイベント
│   │   ├── order/
│   │   │   ├── aggregate.go     # 注文集約
│   │   │   └── events.go        # 注文ドメインイベント
│   │   └── inventory/
│   │       ├── aggregate.go     # 在庫集約
│   │       └── events.go        # 在庫ドメインイベント
│   │
│   ├── projection/              # プロジェクション層
│   │   └── projector.go         # イベント→読み取りモデル変換
│   │
│   ├── notification/            # 通知層
│   │   └── handler.go           # メール通知イベントハンドラー
│   │
│   ├── email/                   # メール送信
│   │   ├── service.go           # SMTPメール送信サービス
│   │   └── templates.go         # HTMLメールテンプレート
│   │
│   └── infrastructure/          # インフラ層
│       ├── kinesis/
│       │   └── record_adapter.go # DynamoDB Streams → Event 変換
│       └── store/
│           ├── interface.go           # Event構造体・EventStoreインターフェース
│           ├── dynamo_event_store.go  # DynamoDB EventStore
│           ├── read_store_interface.go # ReadStoreインターフェース
│           └── postgres_read_store.go  # PostgreSQL Read Store
│
├── infra/                       # インフラ定義
│   └── terraform/               # Terraform IaC
│       ├── main.tf
│       ├── dynamodb.tf
│       ├── kinesis.tf
│       ├── lambda.tf
│       └── cloudwatch.tf
│
├── scripts/                     # スクリプト
│   ├── localstack-init.sh       # LocalStack初期化
│   └── deploy-lambda-local.sh   # Lambda ローカルデプロイ
│
├── frontend/                    # Next.js フロントエンド
│
├── docker-compose.yml           # Docker構成（LocalStack使用）
├── Dockerfile                   # マルチステージビルド
├── init.sql                     # PostgreSQL初期化（read_* テーブル）
├── Makefile                     # タスクランナー
├── go.mod                       # Go モジュール定義
└── go.sum                       # 依存関係ロック
```

### 各層の責務

| 層 | 責務 | ファイル |
|----|------|---------|
| **API層** | HTTPリクエストの受付、レスポンス返却 | `internal/api/` |
| **Command層** | 書き込み操作のハンドリング | `internal/command/` |
| **Query層** | 読み取り操作のハンドリング | `internal/query/` |
| **ReadModel層** | 読み取りモデルの型定義 | `internal/readmodel/` |
| **Domain層** | ビジネスロジック、ドメインイベント定義 | `internal/domain/` |
| **Projection層** | イベントから読み取りモデルを構築 | `internal/projection/` |
| **Notification層** | イベントに基づく通知処理 | `internal/notification/` |
| **Infrastructure層** | 外部サービス連携（Kinesis, DynamoDB, PostgreSQL） | `internal/infrastructure/` |

---

## 技術スタック

| 技術 | 用途 | バージョン |
|------|------|-----------|
| **Go** | バックエンド言語 | 1.24 |
| **AWS Kinesis Data Streams** | イベントストリーミング | - |
| **AWS Lambda** | イベント処理（Projector, Notifier） | provided.al2023 |
| **DynamoDB** | Write DB（イベントストア） | Local / AWS |
| **PostgreSQL** | Read DB（読み取りモデル） | 16 |
| **LocalStack** | ローカルAWSエミュレーション | Latest |
| **Terraform** | Infrastructure as Code | >= 1.0 |
| **Docker** | コンテナ化 | - |

### データベース設計

| DB | テーブル | 用途 |
|----|----------|------|
| **Write DB (DynamoDB)** | `events` | イベントソース（Append-Only） |
| **Write DB (DynamoDB)** | `snapshots` | スナップショット |
| **Read DB (PostgreSQL)** | `read_products` | 商品クエリ用（非正規化） |
| **Read DB (PostgreSQL)** | `read_carts` | カートクエリ用（JSONカラム使用） |
| **Read DB (PostgreSQL)** | `read_orders` | 注文クエリ用（JSONカラム使用） |
| **Read DB (PostgreSQL)** | `read_inventory` | 在庫クエリ用 |

### DynamoDBテーブル設計

```
テーブル名: events
- Partition Key: aggregate_id (String)
- Sort Key: version (Number)

GSI1 (全イベント取得用):
- Partition Key: gsi1pk (固定値 "EVENTS")
- Sort Key: created_at (String - ISO8601)

Kinesis Data Streams 統合:
- DynamoDB への書き込みが自動的に Kinesis へストリーミング
```

### 使用ライブラリ

| ライブラリ | 用途 |
|-----------|------|
| `github.com/aws/aws-lambda-go` | Lambda ランタイム |
| `github.com/aws/aws-sdk-go-v2` | AWS SDK（DynamoDB用） |
| `github.com/lib/pq` | PostgreSQLドライバ |
| `github.com/google/uuid` | UUID生成 |

---

## セットアップ

### 必要条件

- Docker & Docker Compose
- Go 1.24+ （ローカル開発時）
- AWS CLI（状態確認用、オプション）

### 起動方法

```bash
# リポジトリをクローン
cd event-driven-app

# 1. インフラを起動（LocalStack, PostgreSQL, Mailpit）
make infra

# 2. LocalStack の初期化完了を待つ（約20秒）
docker-compose logs -f localstack
# "LocalStack Initialization Complete" が表示されるまで待機

# 3. Lambda関数をビルド＆デプロイ
make deploy-local

# 4. APIサーバーを起動
export JWT_SECRET="change-this-secret-in-production-min-32-chars"
export DYNAMODB_ENDPOINT="http://localhost:4566"
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
make api
```

### Docker Compose で全サービス起動

```bash
# 全サービスを起動（API, Frontend を含む）
make up

# ログを確認
make logs
```

### 環境変数

| 変数名 | 説明 | デフォルト |
|--------|------|------------|
| `DYNAMODB_TABLE_NAME` | DynamoDBテーブル名 | `events` |
| `DYNAMODB_SNAPSHOT_TABLE_NAME` | スナップショットテーブル名 | `snapshots` |
| `DYNAMODB_REGION` | AWSリージョン | `ap-northeast-1` |
| `DYNAMODB_ENDPOINT` | ローカル開発用エンドポイント | (空=AWS本番) |
| `JWT_SECRET` | JWT署名用シークレット（32文字以上） | - |
| `DATABASE_URL` | PostgreSQL接続文字列 | - |

### サービス一覧

| サービス | URL / Port | 説明 |
|---------|------------|------|
| **EC Shop UI** | http://localhost:3000 | Next.js フロントエンド |
| **API Server** | http://localhost:8080 | REST API（Command + Query） |
| **Lambda Projector** | - | Kinesis Consumer（Read DB更新） |
| **Lambda Notifier** | - | Kinesis Consumer（メール送信） |
| **LocalStack** | http://localhost:4566 | AWS サービスエミュレーション |
| **Mailpit** | http://localhost:8025 | 開発用メールサーバ（受信メール確認） |
| **PostgreSQL** | localhost:5432 | Read DB（読み取りモデル） |

### 初期管理者アカウント

| 項目 | 値 |
|------|-----|
| メール | `admin@example.com` |
| パスワード | `admin123` |

### 状態確認コマンド

```bash
# DynamoDB テーブル確認
make dynamodb-status

# Kinesis ストリーム確認
make kinesis-status

# Lambda 関数一覧
make lambda-list

# Lambda Projector ログ
make logs-projector

# Lambda Notifier ログ
make logs-notifier
```

### 停止・クリーンアップ

```bash
# 停止
make down

# 完全クリーンアップ（データ削除）
make clean
```

---

## 使い方

### Web UI での操作

1. http://localhost:3000 にアクセス
2. **Admin** タブで商品を登録
3. **Products** タブで商品をカートに追加
4. **Cart** タブで注文を確定
5. **Orders** タブで注文履歴を確認

### curl での操作

```bash
# 商品登録
curl -X POST http://localhost:8080/products \
  -H "Content-Type: application/json" \
  -d '{"name": "Tシャツ", "description": "綿100%", "price": 2000, "stock": 100}'

# 商品一覧
curl http://localhost:8080/products

# カートに追加
curl -X POST http://localhost:8080/cart/items \
  -H "Content-Type: application/json" \
  -d '{"product_id": "<product_id>", "quantity": 2}'

# カート確認
curl http://localhost:8080/cart

# 注文確定
curl -X POST http://localhost:8080/orders

# 注文一覧
curl http://localhost:8080/orders
```

---

## API リファレンス

### Command API（書き込み）

| メソッド | パス | 説明 | リクエストボディ |
|---------|------|------|-----------------|
| POST | `/products` | 商品登録 | `{name, description, price, stock}` |
| POST | `/cart/items` | カートに追加 | `{product_id, quantity}` |
| DELETE | `/cart/items/{product_id}` | カートから削除 | - |
| POST | `/orders` | 注文確定 | - |
| POST | `/orders/{id}/cancel` | 注文キャンセル | `{reason}` |

### Query API（読み取り）

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/products` | 商品一覧 |
| GET | `/products/{id}` | 商品詳細 |
| GET | `/cart` | カート内容 |
| GET | `/orders` | 注文一覧 |
| GET | `/orders/{id}` | 注文詳細 |

---

## ドメインモデル

### 商品 (Product)

```go
type Product struct {
    ID          string    // 商品ID（UUID）
    Name        string    // 商品名
    Description string    // 説明
    Price       int       // 価格（円）
    Stock       int       // 在庫数
    CreatedAt   time.Time // 作成日時
}
```

### カート (Cart)

```go
type Cart struct {
    ID     string              // カートID
    UserID string              // ユーザーID
    Items  map[string]CartItem // 商品ID → アイテム
}
```

### 注文 (Order)

```go
type Order struct {
    ID        string      // 注文ID（UUID）
    UserID    string      // ユーザーID
    Items     []OrderItem // 注文アイテム
    Total     int         // 合計金額
    Status    Status      // ステータス（pending/paid/shipped/cancelled）
    CreatedAt time.Time   // 作成日時
}
```

### 在庫 (Inventory)

```go
type Inventory struct {
    ProductID     string // 商品ID
    TotalStock    int    // 総在庫
    ReservedStock int    // 予約済み在庫
}
```

---

## イベント一覧

### 商品イベント

| イベント | 発生タイミング | データ |
|---------|---------------|--------|
| `ProductCreated` | 商品登録時 | product_id, name, description, price, stock |
| `ProductUpdated` | 商品更新時 | product_id, name, description, price |
| `ProductDeleted` | 商品削除時 | product_id |

### カートイベント

| イベント | 発生タイミング | データ |
|---------|---------------|--------|
| `ItemAddedToCart` | カート追加時 | cart_id, user_id, product_id, quantity, price |
| `ItemRemovedFromCart` | カート削除時 | cart_id, user_id, product_id |
| `CartCleared` | カートクリア時 | cart_id, user_id |

### 注文イベント

| イベント | 発生タイミング | データ |
|---------|---------------|--------|
| `OrderPlaced` | 注文確定時 | order_id, user_id, items, total |
| `OrderPaid` | 支払い完了時 | order_id |
| `OrderShipped` | 出荷時 | order_id |
| `OrderCancelled` | キャンセル時 | order_id, reason |

**メール通知:** `OrderPlaced` イベント発生時、Lambda Notifier が注文確認メールを送信します。

### 在庫イベント

| イベント | 発生タイミング | データ |
|---------|---------------|--------|
| `StockAdded` | 在庫追加時 | product_id, quantity |
| `StockReserved` | 在庫予約時（注文時） | product_id, order_id, quantity |
| `StockReleased` | 在庫解放時（キャンセル時） | product_id, order_id, quantity |
| `StockDeducted` | 在庫確定時（出荷時） | product_id, order_id, quantity |

---

## データフロー

### 商品登録フロー

```
1. POST /products
       │
       ▼
2. Command Handler
   └─ ProductService.Create()
   └─ InventoryService.AddStock()
       │
       ▼
3. Event Store (DynamoDB)
   └─ PutItem events (ProductCreated, StockAdded)
       │
       ▼ (自動 CDC)
4. Kinesis Data Streams
   └─ DynamoDB Kinesis 統合により自動ストリーミング
       │
       ▼
5. Lambda Projector
   ├─ ProductCreated → INSERT INTO read_products (PostgreSQL)
   └─ StockAdded     → INSERT INTO read_inventory (PostgreSQL)
       │
       ▼
6. Query Handler
   └─ SELECT * FROM read_products (PostgreSQL)
```

### 注文フロー

```
1. POST /orders
       │
       ▼
2. Command Handler
   ├─ カートからアイテム取得（read_carts から）
   ├─ OrderService.Place()       → OrderPlaced イベント
   ├─ InventoryService.Reserve() → StockReserved イベント
   └─ CartService.Clear()        → CartCleared イベント
       │
       ▼
3. Event Store (DynamoDB)
   └─ PutItem events
       │
       ▼ (自動 CDC)
4. Kinesis Data Streams
       │
       ├────────────────────────────┐
       ▼                            ▼
5. Lambda Projector           Lambda Notifier
   ├─ OrderPlaced              └─ OrderPlaced
   │  → INSERT read_orders        → 注文確認メール送信
   ├─ StockReserved
   │  → UPDATE read_inventory
   └─ CartCleared
      → UPDATE read_carts
```

---

## コード解説

### 1. Kinesis Record Adapter (`internal/infrastructure/kinesis/record_adapter.go`)

```go
// DynamoDB Streams 形式のレコードを store.Event に変換
func ConvertFromKinesisRecord(record events.KinesisEventRecord) (*store.Event, error) {
    var dynamoDBRecord events.DynamoDBEventRecord
    json.Unmarshal(record.Kinesis.Data, &dynamoDBRecord)

    // INSERT イベントのみ処理（新規イベント）
    if dynamoDBRecord.EventName != "INSERT" {
        return nil, nil
    }

    return convertDynamoDBImage(dynamoDBRecord.Change.NewImage)
}
```

**ポイント:**
- DynamoDB Kinesis 統合では、DynamoDB Streams 形式でデータが送られる
- `INSERT` イベントのみを処理し、`MODIFY`/`REMOVE` は無視
- Lambda の BatchItemFailures で部分的な再試行が可能

### 2. Lambda Projector (`cmd/lambda/projector/main.go`)

```go
func handler(ctx context.Context, kinesisEvent events.KinesisEvent) (events.KinesisEventResponse, error) {
    var batchItemFailures []events.KinesisBatchItemFailure

    for _, record := range kinesisEvent.Records {
        event, err := kinesis.ConvertFromKinesisRecord(record)
        if err != nil {
            batchItemFailures = append(batchItemFailures, events.KinesisBatchItemFailure{
                ItemIdentifier: record.Kinesis.SequenceNumber,
            })
            continue
        }

        // 既存の Projector を再利用
        eventJSON, _ := json.Marshal(event)
        projector.HandleEvent(ctx, []byte(event.AggregateID), eventJSON)
    }

    return events.KinesisEventResponse{BatchItemFailures: batchItemFailures}, nil
}
```

**ポイント:**
- 既存の `projection.Projector` を Lambda から呼び出し
- `BatchItemFailures` で失敗したレコードのみ再試行

### 3. イベントストア (`internal/infrastructure/store/dynamo_event_store.go`)

```go
// Append stores an event in DynamoDB.
// Events are automatically streamed to Kinesis via DynamoDB Kinesis integration.
func (es *DynamoEventStore) Append(ctx context.Context, aggregateID, aggregateType, eventType string, data any) (*Event, error) {
    // 1. DynamoDB にイベントを保存
    es.client.PutItem(ctx, &dynamodb.PutItemInput{
        TableName:           aws.String(es.tableName),
        Item:                av,
        ConditionExpression: aws.String("attribute_not_exists(aggregate_id) AND attribute_not_exists(version)"),
    })

    // 2. Kinesis へのストリーミングは DynamoDB が自動で行う
    //    アプリケーション側での Publish 処理は不要

    return &event, nil
}
```

**ポイント:**
- **Append-Only** - イベントは追加のみ
- **条件付き書き込み**で楽観的ロック
- **自動CDC** - Kafka Publish 処理が不要になり、コードがシンプルに

---

## 学習ポイント

### 1. 自動CDCの利点を体験する

```bash
# DynamoDB へのイベント書き込みを確認
make dynamodb-status

# Kinesis への自動ストリーミングを確認
make kinesis-status

# Lambda の処理ログを確認
make logs-projector
```

**確認ポイント:**
- DynamoDB への書き込みが自動的に Kinesis へストリーミング
- アプリケーション側でのイベント発行処理が不要

### 2. 結果整合性を体験する

```bash
# 商品を登録
curl -X POST http://localhost:8080/products \
  -H "Content-Type: application/json" \
  -d '{"name": "テスト商品", "price": 1000, "stock": 10}'

# すぐに商品一覧を確認（若干の遅延がある可能性）
curl http://localhost:8080/products
```

**確認ポイント:**
- 書き込み直後は読み取りモデルに**反映されていない可能性**
- Lambda の処理後に**結果整合性**で反映される

### 3. Lambda のスケーラビリティを理解する

```bash
# 複数のイベントを連続で発行
for i in {1..10}; do
  curl -X POST http://localhost:8080/products \
    -H "Content-Type: application/json" \
    -d "{\"name\": \"商品$i\", \"price\": 1000, \"stock\": 10}"
done

# Lambda のログで並列処理を確認
make logs-projector
```

**確認ポイント:**
- Lambda は自動的にスケール
- Kinesis のシャード数に応じた並列処理

---

## 発展的な学習

このプロジェクトをベースに、以下の機能を追加してみましょう：

1. **Saga パターン** - 複数サービス間の分散トランザクション管理
2. **スナップショット最適化** - 大量イベントに対するリプレイの高速化
3. **別種の読み取りDB** - Elasticsearch で全文検索、Redis でキャッシュなど
4. **イベントバージョニング** - イベントスキーマの進化管理
5. **Firehose → S3** - イベントのアーカイブとデータレイク構築
6. **CloudWatch Alarms** - 本番運用のためのモニタリング

---

## ライセンス

MIT License - 学習目的で自由にご利用ください。
