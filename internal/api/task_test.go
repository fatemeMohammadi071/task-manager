package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	mockdb "github.com/fatemeMohammadi071/task-manager/internal/db/mock"
	db "github.com/fatemeMohammadi071/task-manager/internal/db/sqlc"
	"github.com/fatemeMohammadi071/task-manager/util"
)

// ---------- helpers ----------

func randomTask() db.Task {
	assignee := util.RandomAssignee()
	now := time.Now().UTC().Truncate(time.Second)
	return db.Task{
		ID:          util.RandomInt(1, 1000),
		Title:       util.RandomTitle(),
		Description: util.RandomString(30),
		Status:      db.TaskStatusTodo,
		Assignee:    &assignee,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func requireBodyMatchTask(t *testing.T, body *bytes.Buffer, want db.Task) {
	t.Helper()
	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var got db.Task
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, want, got)
}

func requireErrorBody(t *testing.T, body *bytes.Buffer, wantMsg string) {
	t.Helper()
	var got map[string]string
	require.NoError(t, json.Unmarshal(body.Bytes(), &got))
	require.Equal(t, wantMsg, got["error"])
}

// ---------- POST /api/v1/tasks ----------

func TestCreateTaskAPI(t *testing.T) {
	task := randomTask()

	testCases := []struct {
		name          string
		body          gin.H
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: gin.H{
				"title":       task.Title,
				"description": task.Description,
				"status":      string(task.Status),
				"assignee":    *task.Assignee,
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.CreateTaskParams{
					Title:       task.Title,
					Description: task.Description,
					Status:      task.Status,
					Assignee:    task.Assignee,
				}
				store.EXPECT().
					CreateTask(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(task, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
				requireBodyMatchTask(t, recorder.Body, task)
			},
		},
		{
			name: "DefaultStatusIsTodo",
			body: gin.H{
				"title": task.Title, // no status, no assignee
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.CreateTaskParams{
					Title:    task.Title,
					Status:   db.TaskStatusTodo,
					Assignee: nil,
				}
				store.EXPECT().
					CreateTask(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(task, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
			},
		},
		{
			name: "MissingTitle",
			body: gin.H{"description": "no title here"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().CreateTask(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "TitleTooLong",
			body: gin.H{"title": strings.Repeat("a", 256)},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().CreateTask(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidStatus",
			body: gin.H{"title": task.Title, "status": "blocked"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().CreateTask(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InternalError",
			body: gin.H{"title": task.Title},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					CreateTask(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Task{}, errors.New("connection refused"))
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
				// the real DB error must NOT leak to the client
				requireErrorBody(t, recorder.Body, "internal server error")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			req, err := http.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewReader(data))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			server.router.ServeHTTP(recorder, req)
			tc.checkResponse(t, recorder)
		})
	}
}

// ---------- GET /api/v1/tasks/:id ----------

func TestGetTaskAPI(t *testing.T) {
	task := randomTask()

	testCases := []struct {
		name          string
		id            string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			id:   fmt.Sprint(task.ID),
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTask(gomock.Any(), gomock.Eq(task.ID)).
					Times(1).
					Return(task, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				requireBodyMatchTask(t, recorder.Body, task)
			},
		},
		{
			name: "NotFound",
			id:   fmt.Sprint(task.ID),
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTask(gomock.Any(), gomock.Eq(task.ID)).
					Times(1).
					Return(db.Task{}, pgx.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
				requireErrorBody(t, recorder.Body, "task not found")
			},
		},
		{
			name: "InternalError",
			id:   fmt.Sprint(task.ID),
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTask(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Task{}, errors.New("connection refused"))
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
				requireErrorBody(t, recorder.Body, "internal server error")
			},
		},
		{
			name: "InvalidIDZero",
			id:   "0",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().GetTask(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidIDNotNumber",
			id:   "abc",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().GetTask(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := "/api/v1/tasks/" + tc.id
			req, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			server.router.ServeHTTP(recorder, req)
			tc.checkResponse(t, recorder)
		})
	}
}
