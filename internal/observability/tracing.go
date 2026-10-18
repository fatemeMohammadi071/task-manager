package observability

import (
	"context"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// InitTracer sets up OpenTelemetry tracing.
// If endpoint is empty, tracing stays disabled (OTel's no-op tracer is used).
// The returned shutdown function flushes buffered spans; call it on exit.
func InitTracer(ctx context.Context, serviceName, endpoint string) (func(context.Context) error, error) {
	if endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint), // e.g. "jaeger:4318"
		otlptracehttp.WithInsecure(),         // plain HTTP inside the docker network
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter), // send spans in batches, in the background
		sdktrace.WithResource(resource.NewSchemaless(
			attribute.String("service.name", serviceName),
		)),
	)

	otel.SetTracerProvider(tp)
	// propagate trace context to/from other services via the W3C "traceparent" header
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

// TraceIDHeader adds the current trace id to the response as X-Trace-Id.
// Must run AFTER the otelgin middleware.
func TraceIDHeader() gin.HandlerFunc {
	return func(c *gin.Context) {
		if sc := trace.SpanContextFromContext(c.Request.Context()); sc.HasTraceID() {
			c.Header("X-Trace-Id", sc.TraceID().String())
		}
		c.Next()
	}
}
