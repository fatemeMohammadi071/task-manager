package observability

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
)

func restoreGlobalTracer(t *testing.T) {
	prev := otel.GetTracerProvider()
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
}

func TestInitTracerDisabled(t *testing.T) {
	restoreGlobalTracer(t)

	shutdown, err := InitTracer(context.Background(), "test", "")
	require.NoError(t, err)
	require.NoError(t, shutdown(context.Background()))
}

func TestInitTracerEnabled(t *testing.T) {
	restoreGlobalTracer(t)

	// creating the exporter does not connect yet, so no collector is needed
	shutdown, err := InitTracer(context.Background(), "test", "localhost:4318")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, shutdown(ctx))
}
