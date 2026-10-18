package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/mock/gomock"

	mockdb "github.com/fatemeMohammadi071/task-manager/internal/db/mock"
)

// useTestTracer installs an in-memory tracer and returns the recorder of finished spans.
func useTestTracer(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))

	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return recorder
}

func TestTracingCreatesSpanPerRequest(t *testing.T) {
	spans := useTestTracer(t) // must be set BEFORE the server is created

	ctrl := gomock.NewController(t)
	store := mockdb.NewMockStore(ctrl)
	task := randomTask()
	store.EXPECT().GetTask(gomock.Any(), gomock.Any()).Times(1).Return(task, nil)

	server := newTestServer(t, store)
	rec := doRequest(t, server, http.MethodGet, "/api/v1/tasks/1", nil)
	require.Equal(t, http.StatusOK, rec.Code)

	ended := spans.Ended()
	require.Len(t, ended, 1)
	require.Contains(t, ended[0].Name(), "/api/v1/tasks/:id") // route template, not "/1"

	// the trace id is returned to the client for debugging
	require.Equal(t, ended[0].SpanContext().TraceID().String(), rec.Header().Get("X-Trace-Id"))
}

func TestTracingSkipsHealthAndMetrics(t *testing.T) {
	spans := useTestTracer(t)

	ctrl := gomock.NewController(t)
	server := newTestServer(t, mockdb.NewMockStore(ctrl))

	doRequest(t, server, http.MethodGet, "/health", nil)
	doRequest(t, server, http.MethodGet, "/metrics", nil)

	require.Empty(t, spans.Ended())
}
