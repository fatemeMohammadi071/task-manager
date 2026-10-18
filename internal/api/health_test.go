package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	mockdb "github.com/fatemeMohammadi071/task-manager/internal/db/mock"
)

func TestHealth(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := mockdb.NewMockStore(ctrl)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	req, err := http.NewRequest(http.MethodGet, "/health", nil)
	require.NoError(t, err)

	server.Router().ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"status":"ok"}`, recorder.Body.String())
}

func TestSwaggerUI(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := newTestServer(t, mockdb.NewMockStore(ctrl))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil)

	server.Router().ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Task Manager API")
}
