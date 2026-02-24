package main

import (
	"context"
	"log/slog"
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
	"github.com/example/ec-event-driven/internal/observability"
	"github.com/example/ec-event-driven/internal/query"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize structured logging
	observability.InitLogger("ec-api")

	// Initialize OpenTelemetry tracer
	tracerShutdown, err := observability.InitTracer(ctx, observability.TracerConfig{
		ServiceName: "ec-api",
	})
	if err != nil {
		slog.Error("failed to initialize tracer", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := tracerShutdown(ctx); err != nil {
			slog.Error("failed to shutdown tracer", "error", err)
		}
	}()

	// Initialize OpenTelemetry meter
	meterShutdown, err := observability.InitMeter(ctx, observability.MeterConfig{
		ServiceName: "ec-api",
	})
	if err != nil {
		slog.Error("failed to initialize meter", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := meterShutdown(ctx); err != nil {
			slog.Error("failed to shutdown meter", "error", err)
		}
	}()

	// Configuration from environment variables
	postgresConnStr := getEnv("DATABASE_URL", "postgres://ecapp:ecapp@localhost:5432/ecapp?sslmode=disable")
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		slog.Error("JWT_SECRET environment variable is required")
		os.Exit(1)
	}
	if len(jwtSecret) < 32 {
		slog.Error("JWT_SECRET must be at least 32 characters long")
		os.Exit(1)
	}

	// DynamoDB configuration
	dynamoTableName := getEnv("DYNAMODB_TABLE_NAME", "events")
	dynamoSnapshotTableName := getEnv("DYNAMODB_SNAPSHOT_TABLE_NAME", "snapshots")
	dynamoRegion := getEnv("DYNAMODB_REGION", "ap-northeast-1")
	dynamoEndpoint := os.Getenv("DYNAMODB_ENDPOINT")

	slog.Info("EC Shop - CQRS Mode (Kinesis)",
		"write_db", "DynamoDB",
		"read_db", "PostgreSQL",
		"events", "DynamoDB → Kinesis → Lambda",
	)

	// Initialize DynamoDB client
	dynamoClient, err := newDynamoDBClient(ctx, dynamoRegion, dynamoEndpoint)
	if err != nil {
		slog.Error("failed to create DynamoDB client", "error", err)
		os.Exit(1)
	}

	// Initialize DynamoDB EventStore
	eventStore := store.NewDynamoEventStore(dynamoClient, dynamoTableName, dynamoSnapshotTableName)
	slog.Info("event store initialized", "events_table", dynamoTableName, "snapshots_table", dynamoSnapshotTableName)

	// Initialize PostgreSQL connection for read store
	db, err := store.ConnectPostgres(postgresConnStr)
	if err != nil {
		slog.Error("failed to connect to PostgreSQL", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("error closing database", "error", err)
		}
	}()
	slog.Info("connected to PostgreSQL read store")

	// Initialize read store
	readStore := store.NewPostgresReadStore(db)

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
		15*time.Minute,
		7*24*time.Hour,
	)

	// Initialize handlers
	cmdHandler := command.NewHandler(productSvc, cartSvc, orderSvc, inventorySvc, readStore)
	queryHandler := query.NewHandler(readStore)

	slog.Info("read model updates delegated to Lambda Projector via Kinesis")

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
		slog.Info("server started", "addr", ":8080", "mode", "async_projection")
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	slog.Info("shutting down...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("error shutting down server", "error", err)
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
		cfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(region))
		if err != nil {
			return nil, err
		}
		return dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
			o.BaseEndpoint = &endpoint
		}), nil
	}

	cfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	return dynamodb.NewFromConfig(cfg), nil
}
