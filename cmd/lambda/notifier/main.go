package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/example/ec-event-driven/internal/email"
	"github.com/example/ec-event-driven/internal/infrastructure/kinesis"
	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"github.com/example/ec-event-driven/internal/notification"
	"github.com/example/ec-event-driven/internal/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var (
	notificationHandler *notification.Handler
	readStore           *store.PostgresReadStore
	tracer              = otel.Tracer("ec-notifier")
)

var (
	batchSize      metric.Int64Histogram
	eventDuration  metric.Float64Histogram
	eventProcessed metric.Int64Counter
	eventFailure   metric.Int64Counter
)

func init() {
	observability.InitLogger("ec-notifier")

	ctx := context.Background()
	_, err := observability.InitTracer(ctx, observability.TracerConfig{
		ServiceName: "ec-notifier",
	})
	if err != nil {
		slog.Error("failed to initialize tracer", "error", err)
	}

	_, err = observability.InitMeter(ctx, observability.MeterConfig{
		ServiceName: "ec-notifier",
	})
	if err != nil {
		slog.Error("failed to initialize meter", "error", err)
	}

	meter := otel.Meter("ec-notifier")
	batchSize, _ = meter.Int64Histogram("notifier.batch.size",
		metric.WithDescription("Number of records per batch"),
	)
	eventDuration, _ = meter.Float64Histogram("notifier.event.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Event processing duration in seconds"),
	)
	eventProcessed, _ = meter.Int64Counter("notifier.event.processed.total",
		metric.WithDescription("Total number of successfully processed events"),
	)
	eventFailure, _ = meter.Int64Counter("notifier.event.failure.total",
		metric.WithDescription("Total number of failed event processing attempts"),
	)

	postgresConnStr := os.Getenv("DATABASE_URL")
	if postgresConnStr == "" {
		postgresConnStr = "postgres://ecapp:ecapp@localhost:5432/ecapp?sslmode=disable"
	}

	smtpHost := getEnv("SMTP_HOST", "localhost")
	smtpPort := getEnv("SMTP_PORT", "1025")
	smtpFrom := getEnv("SMTP_FROM", "noreply@example.com")

	db, err := store.ConnectPostgres(postgresConnStr)
	if err != nil {
		slog.Error("failed to connect to PostgreSQL", "error", err)
		os.Exit(1)
	}

	readStore = store.NewPostgresReadStore(db)
	emailSvc := email.NewService(smtpHost, smtpPort, smtpFrom)
	notificationHandler = notification.NewHandler(emailSvc, readStore)

	slog.Info("notifier initialized", "smtp_host", smtpHost, "smtp_port", smtpPort)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func handler(ctx context.Context, kinesisEvent events.KinesisEvent) (events.KinesisEventResponse, error) {
	ctx, span := tracer.Start(ctx, "notifier.HandleBatch",
		trace.WithAttributes(attribute.Int("batch.size", len(kinesisEvent.Records))),
	)
	defer span.End()
	defer observability.ForceFlush(ctx)
	defer observability.ForceFlushMetrics(ctx)

	slog.InfoContext(ctx, "received records", "count", len(kinesisEvent.Records))
	batchSize.Record(ctx, int64(len(kinesisEvent.Records)))

	var batchItemFailures []events.KinesisBatchItemFailure

	for _, record := range kinesisEvent.Records {
		event, err := kinesis.ConvertFromKinesisRecord(record)
		if err != nil {
			slog.ErrorContext(ctx, "failed to convert record", "record_id", record.EventID, "error", err)
			batchItemFailures = append(batchItemFailures, events.KinesisBatchItemFailure{
				ItemIdentifier: record.Kinesis.SequenceNumber,
			})
			continue
		}

		if event == nil {
			continue
		}

		eventCtx := observability.ContextFromTraceParent(ctx, event.TraceParent)
		eventCtx, eventSpan := tracer.Start(eventCtx, "notifier.ProcessEvent",
			trace.WithAttributes(
				attribute.String("event.id", event.ID),
				attribute.String("event.type", event.EventType),
			),
		)

		slog.InfoContext(eventCtx, "processing event",
			"event_id", event.ID,
			"event_type", event.EventType,
		)

		metricAttrs := metric.WithAttributes(
			attribute.String("event_type", event.EventType),
			attribute.String("aggregate_type", event.AggregateType),
		)

		eventStart := time.Now()

		eventJSON, err := json.Marshal(event)
		if err != nil {
			slog.ErrorContext(eventCtx, "failed to marshal event", "event_id", event.ID, "error", err)
			eventSpan.End()
			eventFailure.Add(ctx, 1, metricAttrs)
			batchItemFailures = append(batchItemFailures, events.KinesisBatchItemFailure{
				ItemIdentifier: record.Kinesis.SequenceNumber,
			})
			continue
		}

		if err := notificationHandler.HandleEvent(eventCtx, []byte(event.AggregateID), eventJSON); err != nil {
			slog.ErrorContext(eventCtx, "failed to process event", "event_id", event.ID, "error", err)
			eventSpan.End()
			eventFailure.Add(ctx, 1, metricAttrs)
			batchItemFailures = append(batchItemFailures, events.KinesisBatchItemFailure{
				ItemIdentifier: record.Kinesis.SequenceNumber,
			})
			continue
		}

		eventDuration.Record(ctx, time.Since(eventStart).Seconds(), metricAttrs)
		eventProcessed.Add(ctx, 1, metricAttrs)

		eventSpan.End()
	}

	successCount := len(kinesisEvent.Records) - len(batchItemFailures)
	slog.InfoContext(ctx, "batch processing complete",
		"success", successCount,
		"total", len(kinesisEvent.Records),
	)

	return events.KinesisEventResponse{
		BatchItemFailures: batchItemFailures,
	}, nil
}

func main() {
	lambda.Start(handler)
}
