package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	"github.com/example/ec-event-driven/internal/infrastructure/kafka"
	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"github.com/example/ec-event-driven/internal/query"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Configuration from environment variables
	kafkaBrokersStr := getEnv("KAFKA_BROKERS", "localhost:9092")
	kafkaBrokers := strings.Split(kafkaBrokersStr, ",")
	kafkaTopic := getEnv("KAFKA_TOPIC", "ec-events")
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
	log.Println("[API] EC Shop - CQRS Mode")
	log.Println("[API] ========================================")
	log.Printf("[API] Kafka: %v", kafkaBrokers)
	log.Printf("[API] Topic: %s", kafkaTopic)
	log.Println("[API] Write DB: DynamoDB (events table)")
	log.Println("[API] Read DB:  PostgreSQL (read_* tables)")

	// Initialize Kafka producer
	producer := kafka.NewProducer(kafkaBrokers, kafkaTopic)
	defer producer.Close()

	// Initialize DynamoDB client
	dynamoClient, err := newDynamoDBClient(ctx, dynamoRegion, dynamoEndpoint)
	if err != nil {
		log.Fatalf("[API] Failed to create DynamoDB client: %v", err)
	}

	// Initialize DynamoDB EventStore
	eventStore := store.NewDynamoEventStore(dynamoClient, dynamoTableName, dynamoSnapshotTableName, producer)
	log.Printf("[API] Event Store: DynamoDB (events: %s, snapshots: %s)", dynamoTableName, dynamoSnapshotTableName)

	// Initialize PostgreSQL connection for read store
	db, err := store.ConnectPostgres(postgresConnStr)
	if err != nil {
		log.Fatalf("[API] Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()
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

	// Note: Read model updates are handled by the separate Projector service
	// The API only writes events to the event store and publishes to Kafka
	log.Println("[API] Read model updates delegated to Projector service")

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
	server.Shutdown(shutdownCtx)
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
		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(region),
			config.WithEndpointResolverWithOptions(
				aws.EndpointResolverWithOptionsFunc(func(service, reg string, options ...interface{}) (aws.Endpoint, error) {
					return aws.Endpoint{
						URL:           endpoint,
						SigningRegion: reg,
					}, nil
				}),
			),
		)
	} else {
		// Production AWS
		cfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(region))
	}

	if err != nil {
		return nil, err
	}

	return dynamodb.NewFromConfig(cfg), nil
}
