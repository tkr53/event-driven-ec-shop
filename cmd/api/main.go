package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/example/ec-event-driven/internal/api"
	"github.com/example/ec-event-driven/internal/auth"
	"github.com/example/ec-event-driven/internal/command"
	"github.com/example/ec-event-driven/internal/domain/cart"
	"github.com/example/ec-event-driven/internal/domain/inventory"
	"github.com/example/ec-event-driven/internal/domain/order"
	"github.com/example/ec-event-driven/internal/domain/product"
	"github.com/example/ec-event-driven/internal/domain/category"
	"github.com/example/ec-event-driven/internal/domain/user"
	"github.com/example/ec-event-driven/internal/infrastructure/kafka"
	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"github.com/example/ec-event-driven/internal/projection"
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

	log.Println("[API] ========================================")
	log.Println("[API] EC Shop - CQRS Mode")
	log.Println("[API] ========================================")
	log.Printf("[API] Kafka: %v", kafkaBrokers)
	log.Printf("[API] Topic: %s", kafkaTopic)
	log.Println("[API] Write DB: PostgreSQL (events table)")
	log.Println("[API] Read DB:  PostgreSQL (read_* tables)")

	// Initialize Kafka producer
	producer := kafka.NewProducer(kafkaBrokers, kafkaTopic)
	defer producer.Close()

	// Initialize PostgreSQL connection
	db, err := store.ConnectPostgres(postgresConnStr)
	if err != nil {
		log.Fatalf("[API] Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()
	log.Println("[API] Connected to PostgreSQL")

	// Initialize stores
	eventStore := store.NewPostgresEventStore(db, producer)
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

	// Initialize projector
	projector := projection.NewProjector(readStore)

	// Replay existing events from PostgreSQL to build read models
	log.Println("[API] Replaying events from PostgreSQL...")
	replayEvents(eventStore, projector)

	// Start Kafka consumer for new events (async projection)
	consumer := kafka.NewConsumer(kafkaBrokers, kafkaTopic, "api-projector")
	defer consumer.Close()

	// Use WaitGroup to ensure consumer is ready
	var wg sync.WaitGroup
	consumerReady := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("[API] Starting Kafka consumer (async projection)...")
		close(consumerReady) // Signal that consumer is starting
		if err := consumer.Consume(ctx, projector.HandleEvent); err != nil {
			if ctx.Err() == nil {
				log.Printf("[API] Projector error: %v", err)
			}
		}
	}()

	// Wait for consumer to start
	<-consumerReady
	// Give Kafka consumer a moment to establish connection
	time.Sleep(500 * time.Millisecond)
	log.Println("[API] Kafka consumer ready")

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
	cancel() // Cancel context to stop consumer

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	server.Shutdown(shutdownCtx)

	wg.Wait() // Wait for consumer to finish
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// replayEvents replays all events from PostgreSQL to rebuild read models
func replayEvents(eventStore *store.PostgresEventStore, projector *projection.Projector) {
	events := eventStore.GetAllEvents()
	log.Printf("[API] Replaying %d events from event store...", len(events))

	ctx := context.Background()
	for _, event := range events {
		data, _ := event.MarshalJSON()
		if err := projector.HandleEvent(ctx, []byte(event.AggregateID), data); err != nil {
			log.Printf("[API] Error replaying event %s: %v", event.ID, err)
		}
	}
	log.Println("[API] Event replay completed - read models rebuilt")
}
