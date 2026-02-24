package observability

import (
	"context"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// TracerConfig holds configuration for the tracer provider.
type TracerConfig struct {
	ServiceName string
	Endpoint    string // OTLP gRPC endpoint (e.g. "jaeger:4317")
}

// InitTracer initializes the OpenTelemetry TracerProvider.
// Returns a shutdown function that should be called on application exit.
// If OTEL_EXPORTER_OTLP_ENDPOINT is not set, returns a noop shutdown (safe for tests/CI).
func InitTracer(ctx context.Context, cfg TracerConfig) (func(context.Context) error, error) {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	}

	if endpoint == "" {
		// noop: no exporter configured
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

// MeterConfig holds configuration for the meter provider.
type MeterConfig struct {
	ServiceName string
	Endpoint    string // OTLP gRPC endpoint (e.g. "otel-collector:4317")
}

// InitMeter initializes the OpenTelemetry MeterProvider with OTLP gRPC exporter.
// Returns a shutdown function that should be called on application exit.
// If OTEL_EXPORTER_OTLP_ENDPOINT is not set, returns a noop shutdown (safe for tests/CI).
func InitMeter(ctx context.Context, cfg MeterConfig) (func(context.Context) error, error) {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	}

	if endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(endpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
		),
	)
	if err != nil {
		return nil, err
	}

	mp := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(exporter, metric.WithInterval(15*time.Second))),
		metric.WithResource(res),
	)

	otel.SetMeterProvider(mp)

	return mp.Shutdown, nil
}

// ForceFlushMetrics forces the MeterProvider to flush all pending metrics.
// Essential for Lambda to ensure metrics are exported before the environment freezes.
func ForceFlushMetrics(ctx context.Context) error {
	mp, ok := otel.GetMeterProvider().(*metric.MeterProvider)
	if !ok {
		return nil
	}
	return mp.ForceFlush(ctx)
}

// ForceFlush forces the TracerProvider to flush all pending spans.
// This is essential for Lambda: without it, spans from the final
// batch may be lost when the execution environment freezes.
func ForceFlush(ctx context.Context) error {
	tp, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider)
	if !ok {
		return nil // noop provider, nothing to flush
	}
	return tp.ForceFlush(ctx)
}
