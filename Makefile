.PHONY: up down build api projector logs clean

# 全サービス起動（コンテナ）
up:
	docker-compose up -d --build

# 全サービス停止
down:
	docker-compose down

# インフラのみ起動（ローカル開発用）
infra:
	docker-compose up -d zookeeper kafka kafka-ui postgres

# ビルドのみ
build:
	docker-compose build

# ローカルでAPIサーバー起動
api:
	go run cmd/api/main.go

# ローカルでProjector起動
projector:
	go run cmd/projector/main.go

# ログ表示
logs:
	docker-compose logs -f

# API ログのみ
logs-api:
	docker-compose logs -f api

# クリーンアップ（ボリューム含む）
clean:
	docker-compose down -v
	rm -f /api /projector

# テスト用: 商品登録
test-create-product:
	curl -X POST http://localhost:8080/products \
		-H "Content-Type: application/json" \
		-d '{"name": "Tシャツ", "description": "綿100%", "price": 2000, "stock": 100}'

# テスト用: 商品一覧
test-list-products:
	curl http://localhost:8080/products
