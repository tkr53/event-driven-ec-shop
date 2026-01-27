package kafka

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

type MessageHandler func(ctx context.Context, key, value []byte) error

// FailedMessage represents a message that failed processing
type FailedMessage struct {
	Key       []byte    `json:"key"`
	Value     []byte    `json:"value"`
	Error     string    `json:"error"`
	Retries   int       `json:"retries"`
	Timestamp time.Time `json:"timestamp"`
	Topic     string    `json:"topic"`
	Partition int       `json:"partition"`
	Offset    int64     `json:"offset"`
}

type Consumer struct {
	reader    *kafka.Reader
	dlqWriter *kafka.Writer
	maxRetries int
}

// ConsumerOption is a function that configures a Consumer
type ConsumerOption func(*kafka.ReaderConfig)

// WithStartFromLatest configures the consumer to start from the latest offset
// when there's no committed offset for the consumer group
func WithStartFromLatest() ConsumerOption {
	return func(cfg *kafka.ReaderConfig) {
		cfg.StartOffset = kafka.LastOffset
	}
}

func NewConsumer(brokers []string, topic, groupID string, opts ...ConsumerOption) *Consumer {
	cfg := kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	}

	// Apply options
	for _, opt := range opts {
		opt(&cfg)
	}

	reader := kafka.NewReader(cfg)

	// Dead Letter Queue writer for failed messages
	dlqWriter := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic + "-dlq",
		Balancer: &kafka.LeastBytes{},
	}

	return &Consumer{
		reader:     reader,
		dlqWriter:  dlqWriter,
		maxRetries: 3,
	}
}

func (c *Consumer) Consume(ctx context.Context, handler MessageHandler) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				log.Printf("Error reading message: %v", err)
				continue
			}

			// Process message with retry
			if err := c.processWithRetry(ctx, msg, handler); err != nil {
				log.Printf("Message processing failed after retries, sending to DLQ: %v", err)
				c.sendToDLQ(ctx, msg, err)
			}
		}
	}
}

// processWithRetry attempts to process a message with exponential backoff
func (c *Consumer) processWithRetry(ctx context.Context, msg kafka.Message, handler MessageHandler) error {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 100ms, 200ms, 400ms
			backoff := time.Duration(100<<uint(attempt-1)) * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			log.Printf("Retrying message (attempt %d/%d): key=%s", attempt, c.maxRetries, string(msg.Key))
		}

		if err := handler(ctx, msg.Key, msg.Value); err != nil {
			lastErr = err
			log.Printf("Error handling message (attempt %d/%d): %v", attempt+1, c.maxRetries+1, err)
			continue
		}
		return nil // Success
	}
	return lastErr
}

// sendToDLQ sends a failed message to the dead letter queue
func (c *Consumer) sendToDLQ(ctx context.Context, msg kafka.Message, err error) {
	failedMsg := FailedMessage{
		Key:       msg.Key,
		Value:     msg.Value,
		Error:     err.Error(),
		Retries:   c.maxRetries,
		Timestamp: time.Now(),
		Topic:     msg.Topic,
		Partition: msg.Partition,
		Offset:    msg.Offset,
	}

	data, marshalErr := json.Marshal(failedMsg)
	if marshalErr != nil {
		log.Printf("Failed to marshal DLQ message: %v", marshalErr)
		return
	}

	if writeErr := c.dlqWriter.WriteMessages(ctx, kafka.Message{
		Key:   msg.Key,
		Value: data,
	}); writeErr != nil {
		log.Printf("Failed to write to DLQ: %v", writeErr)
	}
}

func (c *Consumer) Close() error {
	if err := c.dlqWriter.Close(); err != nil {
		log.Printf("Error closing DLQ writer: %v", err)
	}
	return c.reader.Close()
}
