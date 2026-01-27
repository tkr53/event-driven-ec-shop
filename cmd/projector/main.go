package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
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

	// DynamoDB configuration (for event replay)
	dynamoTableName := getEnv("DYNAMODB_TABLE_NAME", "events")
	dynamoSnapshotTableName := getEnv("DYNAMODB_SNAPSHOT_TABLE_NAME", "snapshots")
	dynamoRegion := getEnv("DYNAMODB_REGION", "ap-northeast-1")
	dynamoEndpoint := os.Getenv("DYNAMODB_ENDPOINT")

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

	// Initialize DynamoDB client for event replay
	dynamoClient, err := newDynamoDBClient(ctx, dynamoRegion, dynamoEndpoint)
	if err != nil {
		log.Fatalf("[Projector] Failed to create DynamoDB client: %v", err)
	}

	// Initialize DynamoDB EventStore (read-only, no Kafka producer needed)
	eventStore := store.NewDynamoEventStore(dynamoClient, dynamoTableName, dynamoSnapshotTableName, nil)
	log.Printf("[Projector] Event Store: DynamoDB (table: %s)", dynamoTableName)

	// Replay existing events from event store to rebuild read models
	log.Println("[Projector] Replaying events from event store...")
	replayEvents(eventStore, projector)

	// Initialize Kafka consumer with StartFromLatest
	// This ensures we don't re-process events that were already replayed from DynamoDB
	consumer := kafka.NewConsumer(kafkaBrokers, kafkaTopic, consumerGroup, kafka.WithStartFromLatest())
	defer consumer.Close()
	log.Println("[Projector] Kafka consumer configured to start from latest offset")

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

// replayEvents replays all events from the event store to rebuild read models
func replayEvents(eventStore *store.DynamoEventStore, projector *projection.Projector) {
	events := eventStore.GetAllEvents()
	log.Printf("[Projector] Replaying %d events from event store...", len(events))

	ctx := context.Background()
	for _, event := range events {
		data, _ := event.MarshalJSON()
		if err := projector.HandleEvent(ctx, []byte(event.AggregateID), data); err != nil {
			log.Printf("[Projector] Error replaying event %s: %v", event.ID, err)
		}
	}
	log.Println("[Projector] Event replay completed - read models rebuilt")
}
