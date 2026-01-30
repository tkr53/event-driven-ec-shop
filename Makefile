.PHONY: up down build api logs clean test test-coverage test-race \
        build-lambda deploy-local logs-projector logs-notifier

# 全サービス起動（コンテナ）
up:
	docker-compose up -d --build

# 全サービス停止
down:
	docker-compose down

# インフラのみ起動（LocalStack + PostgreSQL + Mailpit）
infra:
	docker-compose up -d localstack postgres mailpit

# ビルドのみ
build:
	docker-compose build

# ローカルでAPIサーバー起動
api:
	go run cmd/api/main.go

# ログ表示
logs:
	docker-compose logs -f

# API ログのみ
logs-api:
	docker-compose logs -f api

# LocalStack ログ
logs-localstack:
	docker-compose logs -f localstack

# クリーンアップ（ボリューム含む）
clean:
	docker-compose down -v
	rm -rf dist/

# ===========================================
# Lambda ビルド・デプロイ
# ===========================================

# Lambda関数をビルド（ARM64）
build-lambda:
	@echo "Building Lambda functions..."
	@mkdir -p dist/lambda/projector dist/lambda/notifier
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o dist/lambda/projector/bootstrap ./cmd/lambda/projector
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o dist/lambda/notifier/bootstrap ./cmd/lambda/notifier
	@echo "Lambda functions built successfully"

# LocalStackにLambda関数をデプロイ
deploy-local: build-lambda
	@echo "Deploying Lambda to LocalStack..."
	./scripts/deploy-lambda-local.sh

# Lambda Projector のCloudWatchログを表示
logs-projector:
	@command -v awslocal >/dev/null 2>&1 && \
		awslocal logs tail /aws/lambda/ec-projector --follow --region ap-northeast-1 2>/dev/null || \
		aws --endpoint-url=http://localhost:4566 logs tail /aws/lambda/ec-projector --follow --region ap-northeast-1 2>/dev/null || \
		echo "No logs yet. Make sure LocalStack is running and Lambda has been invoked."

# Lambda Notifier のCloudWatchログを表示
logs-notifier:
	@command -v awslocal >/dev/null 2>&1 && \
		awslocal logs tail /aws/lambda/ec-notifier --follow --region ap-northeast-1 2>/dev/null || \
		aws --endpoint-url=http://localhost:4566 logs tail /aws/lambda/ec-notifier --follow --region ap-northeast-1 2>/dev/null || \
		echo "No logs yet. Make sure LocalStack is running and Lambda has been invoked."

# ===========================================
# Terraform
# ===========================================

# Terraform 初期化
tf-init:
	cd infra/terraform && terraform init

# Terraform プラン
tf-plan:
	cd infra/terraform && terraform plan

# Terraform 適用
tf-apply:
	cd infra/terraform && terraform apply

# Terraform 破棄
tf-destroy:
	cd infra/terraform && terraform destroy

# ===========================================
# テスト
# ===========================================

# ユニットテスト実行
test:
	go test -v ./...

# カバレッジレポート生成
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# レースコンディション検出
test-race:
	go test -v -race ./...

# ===========================================
# 動作確認
# ===========================================

# テスト用: 商品登録
test-create-product:
	curl -X POST http://localhost:8080/products \
		-H "Content-Type: application/json" \
		-d '{"name": "Tシャツ", "description": "綿100%", "price": 2000, "stock": 100}'

# テスト用: 商品一覧
test-list-products:
	curl http://localhost:8080/products

# Kinesis ストリームの状態確認
kinesis-status:
	@command -v awslocal >/dev/null 2>&1 && \
		awslocal kinesis describe-stream --stream-name ec-events --region ap-northeast-1 || \
		aws --endpoint-url=http://localhost:4566 kinesis describe-stream --stream-name ec-events --region ap-northeast-1

# DynamoDB テーブルの状態確認
dynamodb-status:
	@command -v awslocal >/dev/null 2>&1 && \
		awslocal dynamodb describe-table --table-name events --region ap-northeast-1 || \
		aws --endpoint-url=http://localhost:4566 dynamodb describe-table --table-name events --region ap-northeast-1

# Lambda関数の一覧
lambda-list:
	@command -v awslocal >/dev/null 2>&1 && \
		awslocal lambda list-functions --region ap-northeast-1 || \
		aws --endpoint-url=http://localhost:4566 lambda list-functions --region ap-northeast-1
