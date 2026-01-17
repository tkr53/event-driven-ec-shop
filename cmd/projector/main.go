package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/example/ec-event-driven/internal/infrastructure/kafka"
	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"github.com/example/ec-event-driven/internal/projection"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Configuration from environment variables
	kafkaBrokersStr := getEnv("KAFKA_BROKERS", "localhost:9092")
	kafkaBrokers := strings.Split(kafkaBrokersStr, ",")
	kafkaTopic := getEnv("KAFKA_TOPIC", "ec-events")
	consumerGroup := getEnv("KAFKA_CONSUMER_GROUP", "projector")
	postgresConnStr := getEnv("DATABASE_URL", "postgres://ecapp:ecapp@localhost:5432/ecapp?sslmode=disable")

	log.Println("[Projector] ========================================")
	log.Println("[Projector] EC Shop - CQRS Projector")
	log.Println("[Projector] ========================================")
	log.Printf("[Projector] Kafka: %v", kafkaBrokers)
	log.Printf("[Projector] Topic: %s", kafkaTopic)
	log.Printf("[Projector] Group: %s", consumerGroup)

	// Initialize PostgreSQL connection
	db, err := store.ConnectPostgres(postgresConnStr)
	if err != nil {
		log.Fatalf("[Projector] Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()
	log.Println("[Projector] Connected to PostgreSQL (Read DB)")

	// Initialize read store with PostgreSQL
	readStore := store.NewPostgresReadStore(db)

	// Initialize projector
	projector := projection.NewProjector(readStore)

	// Initialize Kafka consumer
	consumer := kafka.NewConsumer(kafkaBrokers, kafkaTopic, consumerGroup)
	defer consumer.Close()

	// Start consuming
	go func() {
		log.Println("[Projector] Starting event consumer...")
		log.Printf("[Projector] Listening to topic: %s", kafkaTopic)
		if err := consumer.Consume(ctx, projector.HandleEvent); err != nil {
			log.Printf("[Projector] Consumer error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("[Projector] Shutting down...")
	cancel()
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
