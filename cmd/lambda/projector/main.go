package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/example/ec-event-driven/internal/infrastructure/kinesis"
	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"github.com/example/ec-event-driven/internal/projection"
)

var (
	projector *projection.Projector
	readStore *store.PostgresReadStore
)

func init() {
	postgresConnStr := os.Getenv("DATABASE_URL")
	if postgresConnStr == "" {
		postgresConnStr = "postgres://ecapp:ecapp@localhost:5432/ecapp?sslmode=disable"
	}

	db, err := store.ConnectPostgres(postgresConnStr)
	if err != nil {
		log.Fatalf("[Lambda Projector] Failed to connect to PostgreSQL: %v", err)
	}

	readStore = store.NewPostgresReadStore(db)
	projector = projection.NewProjector(readStore)

	log.Println("[Lambda Projector] Initialized successfully")
}

func handler(ctx context.Context, kinesisEvent events.KinesisEvent) (events.KinesisEventResponse, error) {
	log.Printf("[Lambda Projector] Received %d records", len(kinesisEvent.Records))

	var batchItemFailures []events.KinesisBatchItemFailure

	for _, record := range kinesisEvent.Records {
		event, err := kinesis.ConvertFromKinesisRecord(record)
		if err != nil {
			log.Printf("[Lambda Projector] Failed to convert record %s: %v", record.EventID, err)
			batchItemFailures = append(batchItemFailures, events.KinesisBatchItemFailure{
				ItemIdentifier: record.Kinesis.SequenceNumber,
			})
			continue
		}

		// Skip non-INSERT events (e.g., MODIFY, REMOVE)
		if event == nil {
			continue
		}

		log.Printf("[Lambda Projector] Processing event: %s (type: %s, aggregate: %s)",
			event.ID, event.EventType, event.AggregateType)

		// Marshal event to JSON for the projector
		eventJSON, err := json.Marshal(event)
		if err != nil {
			log.Printf("[Lambda Projector] Failed to marshal event %s: %v", event.ID, err)
			batchItemFailures = append(batchItemFailures, events.KinesisBatchItemFailure{
				ItemIdentifier: record.Kinesis.SequenceNumber,
			})
			continue
		}

		// Process the event using existing projector
		if err := projector.HandleEvent(ctx, []byte(event.AggregateID), eventJSON); err != nil {
			log.Printf("[Lambda Projector] Failed to process event %s: %v", event.ID, err)
			batchItemFailures = append(batchItemFailures, events.KinesisBatchItemFailure{
				ItemIdentifier: record.Kinesis.SequenceNumber,
			})
			continue
		}

		log.Printf("[Lambda Projector] Successfully processed event: %s", event.ID)
	}

	successCount := len(kinesisEvent.Records) - len(batchItemFailures)
	log.Printf("[Lambda Projector] Processed %d/%d records successfully", successCount, len(kinesisEvent.Records))

	return events.KinesisEventResponse{
		BatchItemFailures: batchItemFailures,
	}, nil
}

func main() {
	lambda.Start(handler)
}
