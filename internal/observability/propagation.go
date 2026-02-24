package observability

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// mapCarrier implements propagation.TextMapCarrier for a simple map.
type mapCarrier map[string]string

func (c mapCarrier) Get(key string) string    { return c[key] }
func (c mapCarrier) Set(key, value string)     { c[key] = value }
func (c mapCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// InjectTraceParent extracts the W3C traceparent from the current span context
// and returns it as a string. Returns empty string if no active span.
func InjectTraceParent(ctx context.Context) string {
	carrier := make(mapCarrier)
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return carrier["traceparent"]
}

// ContextFromTraceParent restores a context with span context from a W3C traceparent string.
// If traceParent is empty, the original context is returned unchanged.
func ContextFromTraceParent(ctx context.Context, traceParent string) context.Context {
	if traceParent == "" {
		return ctx
	}
	carrier := mapCarrier{"traceparent": traceParent}
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

// TraceIDFromTraceParent extracts just the trace ID from a traceparent string.
// Returns empty string if the traceparent is invalid.
func TraceIDFromTraceParent(traceParent string) string {
	if traceParent == "" {
		return ""
	}
	ctx := ContextFromTraceParent(context.Background(), traceParent)
	sc := trace.SpanContextFromContext(ctx)
	if sc.HasTraceID() {
		return sc.TraceID().String()
	}
	return ""
}
