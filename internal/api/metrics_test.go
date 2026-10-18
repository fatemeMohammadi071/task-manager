package api

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	mockdb "github.com/fatemeMohammadi071/task-manager/internal/db/mock"
	db "github.com/fatemeMohammadi071/task-manager/internal/db/sqlc"
)

func TestTasksCountGauge(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	store.EXPECT().CreateTask(gomock.Any(), gomock.Any()).Times(2).Return(randomTask(), nil)
	store.EXPECT().DeleteTask(gomock.Any(), gomock.Any()).Times(1).Return(int64(1), nil)

	doRequest(t, server, http.MethodPost, "/api/v1/tasks", gin.H{"title": "a"})
	doRequest(t, server, http.MethodPost, "/api/v1/tasks", gin.H{"title": "b"})
	require.Equal(t, 2.0, testutil.ToFloat64(server.metrics.TasksCount))

	doRequest(t, server, http.MethodDelete, "/api/v1/tasks/1", nil)
	require.Equal(t, 1.0, testutil.ToFloat64(server.metrics.TasksCount))
}

func TestTasksCountUnchangedOnFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	store.EXPECT().CreateTask(gomock.Any(), gomock.Any()).Times(1).Return(db.Task{}, errors.New("db down"))
	store.EXPECT().DeleteTask(gomock.Any(), gomock.Any()).Times(1).Return(int64(0), nil) // not found

	doRequest(t, server, http.MethodPost, "/api/v1/tasks", gin.H{"title": "a"})
	doRequest(t, server, http.MethodDelete, "/api/v1/tasks/1", nil)

	require.Equal(t, 0.0, testutil.ToFloat64(server.metrics.TasksCount))
}

func TestMetricsEndpoint(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := newTestServer(t, mockdb.NewMockStore(ctrl))

	doRequest(t, server, http.MethodGet, "/health", nil) // produce one request first
	rec := doRequest(t, server, http.MethodGet, "/metrics", nil)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, `requests_total{method="GET",path="/health",status="200"} 1`)
	require.Contains(t, body, "request_latency_seconds_bucket")
	require.Contains(t, body, "tasks_count 0")
}
