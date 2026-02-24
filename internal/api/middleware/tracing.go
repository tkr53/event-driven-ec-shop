package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

var httpTracer = otel.Tracer("ec-event-driven/http")

var (
	httpRequestDuration metric.Float64Histogram
	httpRequestTotal    metric.Int64Counter
	httpActiveRequests  metric.Int64UpDownCounter
)

func init() {
	meter := otel.Meter("ec-event-driven/http")

	httpRequestDuration, _ = meter.Float64Histogram("http.server.request.duration",
		metric.WithUnit("s"),
		metric.WithDescription("HTTP request duration in seconds"),
	)
	httpRequestTotal, _ = meter.Int64Counter("http.server.request.total",
		metric.WithDescription("Total number of HTTP requests"),
	)
	httpActiveRequests, _ = meter.Int64UpDownCounter("http.server.active_requests",
		metric.WithDescription("Number of active HTTP requests"),
	)
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// TracingMiddleware creates a span for each HTTP request, logs request details, and records metrics.
func TracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		spanName := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		ctx, span := httpTracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.HTTPRequestMethodKey.String(r.Method),
				semconv.URLPath(r.URL.Path),
			),
		)
		defer span.End()

		attrs := metric.WithAttributes(
			attribute.String("method", r.Method),
			attribute.String("path", r.URL.Path),
		)
		httpActiveRequests.Add(ctx, 1, attrs)

		rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		start := time.Now()

		next.ServeHTTP(rec, r.WithContext(ctx))

		duration := time.Since(start)
		span.SetAttributes(
			semconv.HTTPResponseStatusCode(rec.statusCode),
			attribute.Int64("http.duration_ms", duration.Milliseconds()),
		)

		attrsWithStatus := metric.WithAttributes(
			attribute.String("method", r.Method),
			attribute.String("path", r.URL.Path),
			attribute.String("status_code", strconv.Itoa(rec.statusCode)),
		)
		httpRequestDuration.Record(ctx, duration.Seconds(), attrsWithStatus)
		httpRequestTotal.Add(ctx, 1, attrsWithStatus)
		httpActiveRequests.Add(ctx, -1, attrs)

		slog.InfoContext(ctx, "http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.statusCode,
			"duration_ms", duration.Milliseconds(),
		)
	})
}
