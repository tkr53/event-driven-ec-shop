package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/example/ec-event-driven/internal/api"
	"github.com/example/ec-event-driven/internal/auth"
	"github.com/example/ec-event-driven/internal/command"
	"github.com/example/ec-event-driven/internal/domain/cart"
	"github.com/example/ec-event-driven/internal/domain/category"
	"github.com/example/ec-event-driven/internal/domain/inventory"
	"github.com/example/ec-event-driven/internal/domain/order"
	"github.com/example/ec-event-driven/internal/domain/product"
	"github.com/example/ec-event-driven/internal/domain/user"
	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"github.com/example/ec-event-driven/internal/query"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Configuration from environment variables
	postgresConnStr := getEnv("DATABASE_URL", "postgres://ecapp:ecapp@localhost:5432/ecapp?sslmode=disable")
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("[API] JWT_SECRET environment variable is required")
	}
	if len(jwtSecret) < 32 {
		log.Fatal("[API] JWT_SECRET must be at least 32 characters long")
	}

	// DynamoDB configuration
	dynamoTableName := getEnv("DYNAMODB_TABLE_NAME", "events")
	dynamoSnapshotTableName := getEnv("DYNAMODB_SNAPSHOT_TABLE_NAME", "snapshots")
	dynamoRegion := getEnv("DYNAMODB_REGION", "ap-northeast-1")
	dynamoEndpoint := os.Getenv("DYNAMODB_ENDPOINT")

	log.Println("[API] ========================================")
	log.Println("[API] EC Shop - CQRS Mode (Kinesis)")
	log.Println("[API] ========================================")
	log.Println("[API] Write DB: DynamoDB (events table)")
	log.Println("[API] Read DB:  PostgreSQL (read_* tables)")
	log.Println("[API] Events:   DynamoDB → Kinesis → Lambda")

	// Initialize DynamoDB client
	dynamoClient, err := newDynamoDBClient(ctx, dynamoRegion, dynamoEndpoint)
	if err != nil {
		log.Fatalf("[API] Failed to create DynamoDB client: %v", err)
	}

	// Initialize DynamoDB EventStore
	// Events are automatically streamed to Kinesis via DynamoDB Kinesis integration
	eventStore := store.NewDynamoEventStore(dynamoClient, dynamoTableName, dynamoSnapshotTableName)
	log.Printf("[API] Event Store: DynamoDB (events: %s, snapshots: %s)", dynamoTableName, dynamoSnapshotTableName)

	// Initialize PostgreSQL connection for read store
	db, err := store.ConnectPostgres(postgresConnStr)
	if err != nil {
		log.Fatalf("[API] Failed to connect to PostgreSQL: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("[API] Error closing database: %v", err)
		}
	}()
	log.Println("[API] Connected to PostgreSQL (read store)")

	// Initialize read store
	readStore := store.NewPostgresReadStore(db) // Use PostgreSQL for read models

	// Initialize domain services
	productSvc := product.NewService(eventStore)
	cartSvc := cart.NewService(eventStore)
	orderSvc := order.NewService(eventStore)
	inventorySvc := inventory.NewService(eventStore)
	userSvc := user.NewService(eventStore)
	categorySvc := category.NewService(eventStore)

	// Initialize JWT service
	jwtService := auth.NewJWTService(
		jwtSecret,
		15*time.Minute,  // Access token expiry
		7*24*time.Hour,  // Refresh token expiry (7 days)
	)

	// Initialize handlers
	cmdHandler := command.NewHandler(productSvc, cartSvc, orderSvc, inventorySvc, readStore)
	queryHandler := query.NewHandler(readStore)

	// Note: Read model updates are handled by Lambda Projector via Kinesis
	// The API only writes events to DynamoDB; streaming to Kinesis is automatic
	log.Println("[API] Read model updates delegated to Lambda Projector (via Kinesis)")

	// Initialize API
	handlers := api.NewHandlers(cmdHandler, queryHandler)
	authHandlers := api.NewAuthHandlers(userSvc, jwtService, readStore)
	categoryHandlers := api.NewCategoryHandlers(categorySvc, readStore)
	router := api.NewRouter(api.RouterConfig{
		Handlers:         handlers,
		AuthHandlers:     authHandlers,
		CategoryHandlers: categoryHandlers,
		JWTService:       jwtService,
	})

	// Start HTTP server
	server := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	go func() {
		log.Println("[API] ========================================")
		log.Println("[API] Server started on :8080")
		log.Println("[API] ========================================")
		log.Println("[API] Note: Using ASYNC projection")
		log.Println("[API] Read model updates may have slight delay")
		log.Println("[API] ========================================")
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("[API] Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("[API] Shutting down...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[API] Error shutting down server: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// newDynamoDBClient creates a DynamoDB client with optional local endpoint
func newDynamoDBClient(ctx context.Context, region, endpoint string) (*dynamodb.Client, error) {
	var cfg aws.Config
	var err error

	if endpoint != "" {
		// Local development with DynamoDB Local
		cfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(region))
		if err != nil {
			return nil, err
		}
		return dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
			o.BaseEndpoint = &endpoint
		}), nil
	}

	// Production AWS
	cfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	return dynamodb.NewFromConfig(cfg), nil
}
