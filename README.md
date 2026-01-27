# EC Shop - Event Driven + CQRS Learning Project

Go + Apache Kafka + PostgreSQL を使った**イベント駆動アーキテクチャ**と**CQRS（Command Query Responsibility Segregation）パターン**を学ぶためのECサイトバックエンドです。

**特徴:**
- **書き込みDB**と**読み取りDB**を完全分離したCQRS実装
- **非同期プロジェクション**によるイベント駆動の読み取りモデル更新
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
│                              http://localhost:8080                           │
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
             ▼                                             │
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Apache Kafka                                    │
│                         Topic: ec-events                                     │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  ProductCreated → StockAdded → ItemAddedToCart → OrderPlaced → ...  │    │
│  └────────────────────────────────────┬────────────────────────────────┘    │
└───────────────────────────────────────┼─────────────────────────────────────┘
                                        │
                                        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Projector                                       │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  Kafka Consumer → Event Handler → PostgreSQL (Read Tables)          │    │
│  │  consumer group: "projector"                                         │    │
│  │                                                                      │    │
│  │  ProductCreated → read_products に INSERT                           │    │
│  │  StockAdded     → read_inventory に INSERT/UPDATE                   │    │
│  │  OrderPlaced    → read_orders に INSERT                             │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────────┘
                                       │
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Notifier (Email)                                │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  Kafka Consumer → Event Handler → SMTP (Mailpit)                    │    │
│  │  consumer group: "email-notifier"                                    │    │
│  │                                                                      │    │
│  │  OrderPlaced → 注文確認メール送信                                     │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────────┘
```

### DB構成（CQRS分離）

| 用途 | ストレージ | 説明 |
|------|----------|------|
| **Write DB** | DynamoDB `events` | イベントストア（追記専用） |
| **Read DB** | PostgreSQL `read_products` | 商品一覧クエリ用 |
| **Read DB** | PostgreSQL `read_carts` | カート情報クエリ用 |
| **Read DB** | PostgreSQL `read_orders` | 注文履歴クエリ用 |
| **Read DB** | PostgreSQL `read_inventory` | 在庫情報クエリ用 |

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
                                                          │
                                                          ▼
                                                 ┌──────────────────┐
                                                 │     Kafka        │
                                                 │  (非同期配信)     │
                                                 └────────┬─────────┘
                                                          │
                                                          ▼
                                                 ┌──────────────────┐
                                                 │   Projector      │
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
│   ├── projector/
│   │   └── main.go              # Projectorワーカーのメイン（独立プロセス）
│   └── notifier/
│       └── main.go              # Email Notifierのメイン（独立プロセス）
│
├── internal/                     # 内部パッケージ
│   ├── api/                     # HTTP API層
│   │   ├── handlers.go          # HTTPハンドラー（リクエスト/レスポンス処理）
│   │   └── router.go            # ルーティング設定
│   │
│   ├── command/                 # コマンド層（CQRS の Command側）
│   │   ├── commands.go          # コマンド定義（DTO）
│   │   └── handler.go           # コマンドハンドラー（ビジネスロジック）
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
│   │   │   ├── aggregate.go     # 商品集約（エンティティ + ビジネスルール）
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
│       ├── kafka/
│       │   ├── producer.go      # Kafkaプロデューサー
│       │   └── consumer.go      # Kafkaコンシューマー
│       └── store/
│           ├── interface.go           # Event構造体・EventStoreインターフェース
│           ├── dynamo_event_store.go   # DynamoDB EventStore
│           ├── read_store.go          # インメモリRead Store（開発用）
│           ├── read_store_interface.go # ReadStoreインターフェース
│           └── postgres_read_store.go  # PostgreSQL Read Store（本番用）
│
├── web/                         # フロントエンド
│   └── index.html               # ECサイトUI（SPA）
│
├── docker-compose.yml           # Docker構成
├── Dockerfile                   # マルチステージビルド
├── init.sql                     # PostgreSQL初期化（read_* テーブル）
├── init-dynamodb.sh             # DynamoDB Local初期化スクリプト
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
| **Email層** | メール送信サービス | `internal/email/` |
| **Infrastructure層** | 外部サービス連携（Kafka, PostgreSQL） | `internal/infrastructure/` |

---

## 技術スタック

| 技術 | 用途 | バージョン |
|------|------|-----------|
| **Go** | バックエンド言語 | 1.24 |
| **Apache Kafka** | メッセージブローカー（イベント配信） | 7.5.0 (Confluent) |
| **DynamoDB** | Write DB（イベントストア） | Local / AWS |
| **PostgreSQL** | Read DB（読み取りモデル） | 16 |
| **Docker** | コンテナ化 | - |

### データベース設計

| DB | テーブル | 用途 |
|----|----------|------|
| **Write DB (DynamoDB)** | `events` | イベントソース（Append-Only） |
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
```

### 使用ライブラリ

| ライブラリ | 用途 |
|-----------|------|
| `github.com/segmentio/kafka-go` | Kafkaクライアント |
| `github.com/lib/pq` | PostgreSQLドライバ |
| `github.com/google/uuid` | UUID生成 |
| `github.com/aws/aws-sdk-go-v2` | AWS SDK（DynamoDB用） |

---

## セットアップ

### 必要条件

- Docker & Docker Compose
- Go 1.23+ （ローカル開発時）

### 起動方法

```bash
# リポジトリをクローン
cd event-driven-app

# 1. インフラを起動（Kafka, PostgreSQL, DynamoDB）
make infra

# 2. DynamoDB テーブルを初期化（初回のみ、起動完了を待機）
./init-dynamodb.sh

# 3. 全サービスを起動
make up

# ログを確認
make logs
```

### ローカル開発（go runで起動）

```bash
# 1. インフラを起動
make infra

# 2. DynamoDB テーブルを初期化（初回のみ）
./init-dynamodb.sh

# 3. APIを起動（go run）
DYNAMODB_ENDPOINT=http://localhost:8000 \
JWT_SECRET=your-secret-key-at-least-32-characters \
go run cmd/api/main.go
```

### 環境変数

| 変数名 | 説明 | デフォルト |
|--------|------|------------|
| `DYNAMODB_TABLE_NAME` | DynamoDBテーブル名 | `events` |
| `DYNAMODB_REGION` | AWSリージョン | `ap-northeast-1` |
| `DYNAMODB_ENDPOINT` | ローカル開発用エンドポイント | (空=AWS本番) |

### DynamoDB Admin UI

DynamoDB Localのデータを確認するには http://localhost:8001 にアクセスしてください。

### サービス一覧

| サービス | URL / Port | 説明 |
|---------|------------|------|
| **EC Shop UI** | http://localhost:3000 | Next.js フロントエンド |
| **API Server** | http://localhost:8080 | REST API（Command + Query） |
| **Projector** | - | Kafkaコンシューマー（Read DB更新） |
| **Notifier** | - | Kafkaコンシューマー（メール送信） |
| **Kafka UI** | http://localhost:8081 | Kafkaモニタリング |
| **Mailpit** | http://localhost:8025 | 開発用メールサーバ（受信メール確認） |
| **DynamoDB Local** | http://localhost:8000 | Write DB（イベントストア） |
| **DynamoDB Admin** | http://localhost:8001 | DynamoDBモニタリングUI |
| **PostgreSQL** | localhost:5432 | Read DB（読み取りモデル） |

### 初期管理者アカウント

| 項目 | 値 |
|------|-----|
| メール | `admin@example.com` |
| パスワード | `admin123` |

管理画面（http://localhost:3000/admin）にアクセスするには、上記アカウントでログインしてください。

**注意:** 本番環境ではパスワードを変更してください。

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

1. http://localhost:8080 にアクセス
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

# 注文キャンセル
curl -X POST http://localhost:8080/orders/<order_id>/cancel \
  -H "Content-Type: application/json" \
  -d '{"reason": "気が変わった"}'
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

type CartItem struct {
    ProductID string // 商品ID
    Quantity  int    // 数量
    Price     int    // 単価
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

**メール通知:** `OrderPlaced` イベント発生時、Notifierサービスが注文確認メールを送信します。開発環境では Mailpit (http://localhost:8025) で受信メールを確認できます。

### 在庫イベント

| イベント | 発生タイミング | データ |
|---------|---------------|--------|
| `StockAdded` | 在庫追加時 | product_id, quantity |
| `StockReserved` | 在庫予約時（注文時） | product_id, order_id, quantity |
| `StockReleased` | 在庫解放時（キャンセル時） | product_id, order_id, quantity |
| `StockDeducted` | 在庫確定時（出荷時） | product_id, order_id, quantity |

---

## データフロー

### 商品登録フロー（非同期プロジェクション）

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
       ▼
4. Kafka Producer
   └─ Topic: ec-events に発行
       │
       ▼
5. Kafka Consumer (Projector)
   └─ イベントを受信
       │
       ▼
6. Projector
   ├─ ProductCreated → INSERT INTO read_products (PostgreSQL)
   └─ StockAdded     → INSERT INTO read_inventory (PostgreSQL)
       │
       ▼
7. Query Handler
   └─ SELECT * FROM read_products (PostgreSQL)
```

### 注文フロー（非同期プロジェクション）

```
1. POST /orders
       │
       ▼
2. Command Handler
   ├─ カートからアイテム取得（read_carts から）
   ├─ OrderService.Place()       → OrderPlaced イベント
   ├─ InventoryService.Reserve() → StockReserved イベント (各アイテム)
   └─ CartService.Clear()        → CartCleared イベント
       │
       ▼
3. Event Store (DynamoDB)
   └─ PutItem events (OrderPlaced, StockReserved, CartCleared)
       │
       ▼
4. Kafka Producer
   └─ 各イベントを Topic: ec-events に発行
       │
       ▼
5. Projector (非同期)
   ├─ OrderPlaced    → INSERT INTO read_orders (PostgreSQL)
   ├─ StockReserved  → UPDATE read_inventory (PostgreSQL)
   │                 → UPDATE read_products (PostgreSQL)
   └─ CartCleared    → UPDATE read_carts (PostgreSQL)
```

### イベントリプレイ（起動時）

```
API Server 起動
       │
       ▼
Event Store から全イベント取得
   └─ DynamoDB Query (GSI1: gsi1pk = "EVENTS")
       │
       ▼
Projector でイベントをリプレイ
   └─ 各イベントを順番に処理
       │
       ▼
Read DB (PostgreSQL) が再構築される
   └─ read_products, read_carts, read_orders, read_inventory
       │
       ▼
Kafka Consumer 開始
   └─ 新規イベントをリアルタイムで処理
```

---

## コード解説

### 1. イベントの定義 (`internal/domain/product/events.go`)

```go
// イベントタイプの定数
const (
    EventProductCreated = "ProductCreated"
    EventProductUpdated = "ProductUpdated"
    EventProductDeleted = "ProductDeleted"
)

// イベントデータ構造体
type ProductCreated struct {
    ProductID   string    `json:"product_id"`
    Name        string    `json:"name"`
    Description string    `json:"description"`
    Price       int       `json:"price"`
    Stock       int       `json:"stock"`
    CreatedAt   time.Time `json:"created_at"`
}
```

**ポイント:**
- イベントは**過去形**で命名（Created, Updated, Deleted）
- イベントは**不変（イミュータブル）** - 一度作成したら変更しない
- イベントには**発生時刻**を含める

### 2. 集約（Aggregate）の実装 (`internal/domain/product/aggregate.go`)

```go
type Service struct {
    eventStore store.EventStoreInterface
}

func (s *Service) Create(ctx context.Context, name, description string, price, stock int) (*Product, error) {
    // 1. バリデーション
    if name == "" {
        return nil, ErrInvalidName
    }
    if price <= 0 {
        return nil, ErrInvalidPrice
    }

    // 2. 新しいIDを生成
    productID := uuid.New().String()

    // 3. イベントを作成
    event := ProductCreated{
        ProductID:   productID,
        Name:        name,
        Description: description,
        Price:       price,
        Stock:       stock,
        CreatedAt:   time.Now(),
    }

    // 4. イベントを保存（Event Store → Kafka）
    _, err := s.eventStore.Append(ctx, productID, AggregateType, EventProductCreated, event)
    if err != nil {
        return nil, err
    }

    // 5. 結果を返す
    return &Product{
        ID:    productID,
        Name:  name,
        Price: price,
        Stock: stock,
    }, nil
}
```

**ポイント:**
- 集約は**ビジネスルール**を守る責務を持つ
- 状態変更は必ず**イベントを発行**して行う
- イベントを保存すると**自動的にKafkaに発行**される

### 3. コマンドハンドラー (`internal/command/handler.go`)

```go
func (h *Handler) CreateProduct(ctx context.Context, cmd CreateProduct) (*product.Product, error) {
    // 1. ドメインサービスを呼び出し（ProductCreatedイベント発行）
    p, err := h.productSvc.Create(ctx, cmd.Name, cmd.Description, cmd.Price, cmd.Stock)
    if err != nil {
        return nil, err
    }

    // 2. 在庫を初期化（StockAddedイベント発行）
    if err := h.inventorySvc.AddStock(ctx, p.ID, cmd.Stock); err != nil {
        return nil, err
    }

    // Read Store は Kafka Consumer (Projector) 経由で非同期更新される
    // ここでは読み取りモデルを直接更新しない

    return p, nil
}
```

**ポイント:**
- コマンドハンドラーは**ユースケース**を実装
- 複数の集約をまたぐ操作を**調整（オーケストレーション）**
- **非同期プロジェクション** - 読み取りモデルはKafka経由で更新
- 書き込みと読み取りの**完全分離**

### 4. プロジェクター (`internal/projection/projector.go`)

```go
func (p *Projector) HandleEvent(ctx context.Context, key, value []byte) error {
    var event store.Event
    json.Unmarshal(value, &event)

    log.Printf("[Projector] Received event: %s (aggregate: %s)",
        event.EventType, event.AggregateType)

    // イベントタイプに応じて読み取りモデルを更新
    switch event.AggregateType {
    case product.AggregateType:
        return p.handleProductEvent(event)
    case cart.AggregateType:
        return p.handleCartEvent(event)
    case order.AggregateType:
        return p.handleOrderEvent(event)
    case inventory.AggregateType:
        return p.handleInventoryEvent(event)
    }
    return nil
}

func (p *Projector) handleProductEvent(event store.Event) error {
    switch event.EventType {
    case product.EventProductCreated:
        var e product.ProductCreated
        json.Unmarshal(event.Data, &e)

        // PostgreSQL の read_products テーブルに INSERT/UPDATE
        p.readStore.Set("products", e.ProductID, &readmodel.ProductReadModel{
            ID:          e.ProductID,
            Name:        e.Name,
            Description: e.Description,
            Price:       e.Price,
            Stock:       e.Stock,
            CreatedAt:   e.CreatedAt,
            UpdatedAt:   e.CreatedAt,
        })
    }
    return nil
}
```

**ポイント:**
- プロジェクターは**Kafkaからイベントを購読**
- **イベントタイプごと**に処理を分岐
- 読み取りモデルは**PostgreSQLに永続化**
- ReadStoreInterface により**インメモリ/PostgreSQLを切り替え可能**

### 5. PostgreSQL Read Store (`internal/infrastructure/store/postgres_read_store.go`)

```go
// ReadStoreInterface を実装
type PostgresReadStore struct {
    db *sql.DB
}

func (rs *PostgresReadStore) Set(collection, id string, data any) {
    switch collection {
    case "products":
        p := data.(*readmodel.ProductReadModel)
        rs.db.Exec(`
            INSERT INTO read_products (id, name, description, price, stock, created_at, updated_at)
            VALUES ($1, $2, $3, $4, $5, $6, $7)
            ON CONFLICT (id) DO UPDATE SET
                name = EXCLUDED.name,
                description = EXCLUDED.description,
                price = EXCLUDED.price,
                stock = EXCLUDED.stock,
                updated_at = EXCLUDED.updated_at
        `, p.ID, p.Name, p.Description, p.Price, p.Stock, p.CreatedAt, p.UpdatedAt)
    // ...他のコレクションも同様
    }
}
```

**ポイント:**
- `ON CONFLICT DO UPDATE` で **UPSERT** を実現
- 読み取りモデルは**クエリに最適化**された形式
- インターフェースにより**テスト時はインメモリ**に差し替え可能

### 5. イベントストア (`internal/infrastructure/store/dynamo_event_store.go`)

```go
func (es *DynamoEventStore) Append(ctx context.Context, aggregateID, aggregateType, eventType string, data any) (*Event, error) {
    // 1. JSONシリアライズ
    jsonData, _ := json.Marshal(data)

    // 2. 次のバージョン番号を取得
    version, _ := es.getNextVersion(ctx, aggregateID)

    // 3. DynamoDBアイテムを作成
    item := dynamoEvent{
        AggregateID:   aggregateID,
        Version:       version,
        ID:            uuid.New().String(),
        AggregateType: aggregateType,
        EventType:     eventType,
        Data:          string(jsonData),
        CreatedAt:     time.Now().Format(time.RFC3339Nano),
        GSI1PK:        "EVENTS", // GetAllEvents用のGSI
    }

    // 4. DynamoDBに保存（条件付き書き込みで楽観的ロック）
    es.client.PutItem(ctx, &dynamodb.PutItemInput{
        TableName:           aws.String(es.tableName),
        Item:                av,
        ConditionExpression: aws.String("attribute_not_exists(aggregate_id) AND attribute_not_exists(version)"),
    })

    // 5. Kafkaに発行
    es.producer.Publish(ctx, aggregateID, event)

    return &event, nil
}
```

**ポイント:**
- **Append-Only** - イベントは追加のみ、更新・削除しない
- **条件付き書き込み**で楽観的ロックを実現（重複バージョン防止）
- **GSI1** でGetAllEvents（全イベント取得）を効率的に実行
- DynamoDB保存 → Kafka発行の**二重書き込み**

---

## 学習ポイント

### 1. イベント駆動の利点を体験する

```bash
# イベントストア（Write DB - DynamoDB）を確認
# DynamoDB Admin UI (http://localhost:8001) でeventsテーブルを確認

# 読み取りモデル（Read DB - PostgreSQL）を確認
docker-compose exec postgres psql -U ecapp -c "SELECT * FROM read_products;"
docker-compose exec postgres psql -U ecapp -c "SELECT * FROM read_inventory;"
```

**確認ポイント:**
- すべての操作が**イベントとしてDynamoDBに記録**されている
- 読み取りモデルは**PostgreSQLの別テーブル**に最適化された形式で保存

### 2. 結果整合性を体験する

このプロジェクトでは**非同期プロジェクション**を採用しています。

```bash
# 商品を登録
curl -X POST http://localhost:8080/products \
  -H "Content-Type: application/json" \
  -d '{"name": "テスト商品", "price": 1000, "stock": 10}'

# すぐに商品一覧を確認（若干の遅延がある可能性）
curl http://localhost:8080/products
```

**確認ポイント:**
- 書き込み直後は読み取りモデルに**反映されていない可能性**がある
- 数百ミリ秒後には**結果整合性**で反映される

### 3. イベントリプレイを試す

```bash
# 読み取りテーブルをクリア
docker-compose exec postgres psql -U ecapp -c "TRUNCATE read_products, read_carts, read_orders, read_inventory;"

# APIを再起動（イベントリプレイが実行される）
docker-compose restart api

# ログを確認
docker-compose logs api | grep "Replaying"
# → [API] Replaying 5 events from event store...
# → [API] Event replay completed - read models rebuilt

# 読み取りモデルが復元されていることを確認
docker-compose exec postgres psql -U ecapp -c "SELECT * FROM read_products;"
```

**確認ポイント:**
- イベントストアから**過去のイベントを再生**
- 読み取りモデルが**完全に再構築**される

### 4. 書き込みDBと読み取りDBの分離を確認

```bash
# 書き込みDB（DynamoDB events）のレコード数
# DynamoDB Admin UI (http://localhost:8001) でeventsテーブルのアイテム数を確認

# 読み取りDB（PostgreSQL read_products）のレコード数
docker-compose exec postgres psql -U ecapp -c "SELECT COUNT(*) FROM read_products;"
```

**確認ポイント:**
- DynamoDB eventsには**すべてのイベント**（更新・削除含む）
- PostgreSQL read_productsには**現在の状態のみ**

### 5. スケーラビリティを理解する

```bash
# Projectorを複数起動（Kafkaコンシューマーグループで分散処理）
docker-compose up -d --scale projector=2
```

**確認ポイント:**
- 複数のProjectorが**同じ読み取りDBを更新**
- Kafkaコンシューマーグループにより**負荷分散**

---

## 発展的な学習

このプロジェクトをベースに、以下の機能を追加してみましょう：

1. **Saga パターン** - 複数サービス間の分散トランザクション管理
2. **Outbox パターン** - イベント発行の信頼性向上（二重書き込み問題の解決）
3. **スナップショット** - イベントリプレイの高速化（大量イベント対策）
4. **別種の読み取りDB** - Elasticsearchで全文検索、Redisでキャッシュなど
5. **イベントバージョニング** - イベントスキーマの進化管理
6. **Idempotency（冪等性）** - 同じイベントの重複処理防止

---

## ライセンス

MIT License - 学習目的で自由にご利用ください。
