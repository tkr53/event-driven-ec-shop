package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/example/ec-event-driven/internal/email"
	"github.com/example/ec-event-driven/internal/infrastructure/kafka"
	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"github.com/example/ec-event-driven/internal/notification"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Configuration from environment variables
	kafkaBrokersStr := getEnv("KAFKA_BROKERS", "localhost:9092")
	kafkaBrokers := strings.Split(kafkaBrokersStr, ",")
	kafkaTopic := getEnv("KAFKA_TOPIC", "ec-events")
	consumerGroup := "email-notifier" // Dedicated consumer group for email notifications

	smtpHost := getEnv("SMTP_HOST", "localhost")
	smtpPort := getEnv("SMTP_PORT", "1025")
	smtpFrom := getEnv("SMTP_FROM", "noreply@example.com")

	postgresConnStr := getEnv("DATABASE_URL", "postgres://ecapp:ecapp@localhost:5432/ecapp?sslmode=disable")

	log.Println("[Notifier] ========================================")
	log.Println("[Notifier] EC Shop - Email Notification Service")
	log.Println("[Notifier] ========================================")
	log.Printf("[Notifier] Kafka: %v", kafkaBrokers)
	log.Printf("[Notifier] Topic: %s", kafkaTopic)
	log.Printf("[Notifier] Group: %s", consumerGroup)
	log.Printf("[Notifier] SMTP: %s:%s", smtpHost, smtpPort)
	log.Printf("[Notifier] From: %s", smtpFrom)

	// Initialize PostgreSQL connection (for reading user data)
	db, err := store.ConnectPostgres(postgresConnStr)
	if err != nil {
		log.Fatalf("[Notifier] Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()
	log.Println("[Notifier] Connected to PostgreSQL (Read DB)")

	// Initialize read store with PostgreSQL
	readStore := store.NewPostgresReadStore(db)

	// Initialize email service
	emailSvc := email.NewService(smtpHost, smtpPort, smtpFrom)

	// Initialize notification handler
	handler := notification.NewHandler(emailSvc, readStore)

	// Initialize Kafka consumer
	consumer := kafka.NewConsumer(kafkaBrokers, kafkaTopic, consumerGroup)
	defer consumer.Close()

	// Start consuming
	go func() {
		log.Println("[Notifier] Starting event consumer...")
		log.Printf("[Notifier] Listening to topic: %s", kafkaTopic)
		if err := consumer.Consume(ctx, handler.HandleEvent); err != nil {
			log.Printf("[Notifier] Consumer error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("[Notifier] Shutting down...")
	cancel()
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
