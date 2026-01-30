package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/example/ec-event-driven/internal/email"
	"github.com/example/ec-event-driven/internal/infrastructure/kinesis"
	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"github.com/example/ec-event-driven/internal/notification"
)

var (
	notificationHandler *notification.Handler
	readStore           *store.PostgresReadStore
)

func init() {
	postgresConnStr := os.Getenv("DATABASE_URL")
	if postgresConnStr == "" {
		postgresConnStr = "postgres://ecapp:ecapp@localhost:5432/ecapp?sslmode=disable"
	}

	smtpHost := getEnv("SMTP_HOST", "localhost")
	smtpPort := getEnv("SMTP_PORT", "1025")
	smtpFrom := getEnv("SMTP_FROM", "noreply@example.com")

	db, err := store.ConnectPostgres(postgresConnStr)
	if err != nil {
		log.Fatalf("[Lambda Notifier] Failed to connect to PostgreSQL: %v", err)
	}

	readStore = store.NewPostgresReadStore(db)
	emailSvc := email.NewService(smtpHost, smtpPort, smtpFrom)
	notificationHandler = notification.NewHandler(emailSvc, readStore)

	log.Printf("[Lambda Notifier] Initialized successfully (SMTP: %s:%s)", smtpHost, smtpPort)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func handler(ctx context.Context, kinesisEvent events.KinesisEvent) (events.KinesisEventResponse, error) {
	log.Printf("[Lambda Notifier] Received %d records", len(kinesisEvent.Records))

	var batchItemFailures []events.KinesisBatchItemFailure

	for _, record := range kinesisEvent.Records {
		event, err := kinesis.ConvertFromKinesisRecord(record)
		if err != nil {
			log.Printf("[Lambda Notifier] Failed to convert record %s: %v", record.EventID, err)
			batchItemFailures = append(batchItemFailures, events.KinesisBatchItemFailure{
				ItemIdentifier: record.Kinesis.SequenceNumber,
			})
			continue
		}

		// Skip non-INSERT events
		if event == nil {
			continue
		}

		log.Printf("[Lambda Notifier] Processing event: %s (type: %s)", event.ID, event.EventType)

		// Marshal event to JSON for the notification handler
		eventJSON, err := json.Marshal(event)
		if err != nil {
			log.Printf("[Lambda Notifier] Failed to marshal event %s: %v", event.ID, err)
			batchItemFailures = append(batchItemFailures, events.KinesisBatchItemFailure{
				ItemIdentifier: record.Kinesis.SequenceNumber,
			})
			continue
		}

		// Process the event using existing notification handler
		if err := notificationHandler.HandleEvent(ctx, []byte(event.AggregateID), eventJSON); err != nil {
			log.Printf("[Lambda Notifier] Failed to process event %s: %v", event.ID, err)
			batchItemFailures = append(batchItemFailures, events.KinesisBatchItemFailure{
				ItemIdentifier: record.Kinesis.SequenceNumber,
			})
			continue
		}
	}

	successCount := len(kinesisEvent.Records) - len(batchItemFailures)
	log.Printf("[Lambda Notifier] Processed %d/%d records successfully", successCount, len(kinesisEvent.Records))

	return events.KinesisEventResponse{
		BatchItemFailures: batchItemFailures,
	}, nil
}

func main() {
	lambda.Start(handler)
}
