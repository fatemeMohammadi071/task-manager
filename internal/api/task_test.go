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

// ---------- GET /api/v1/tasks ----------

type listResponse struct {
	Data     []db.Task `json:"data"`
	Total    int64     `json:"total"`
	PageID   int32     `json:"page_id"`
	PageSize int32     `json:"page_size"`
}

func requireBodyMatchList(t *testing.T, body *bytes.Buffer, want listResponse) {
	t.Helper()
	var got listResponse
	require.NoError(t, json.Unmarshal(body.Bytes(), &got))
	require.Equal(t, want, got)
}

func TestListTasksAPI(t *testing.T) {
	tasks := []db.Task{randomTask(), randomTask(), randomTask()}

	done := db.TaskStatusDone
	assignee := "ali"

	testCases := []struct {
		name          string
		query         string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OKDefaults",
			query: "",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListTasks(gomock.Any(), gomock.Eq(db.ListTasksParams{Limit: 10, Offset: 0})).
					Times(1).
					Return(tasks, nil)
				store.EXPECT().
					CountTasks(gomock.Any(), gomock.Eq(db.CountTasksParams{})).
					Times(1).
					Return(int64(3), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				requireBodyMatchList(t, recorder.Body, listResponse{
					Data: tasks, Total: 3, PageID: 1, PageSize: 10,
				})
			},
		},
		{
			name:  "OKWithFiltersAndPage2",
			query: "?status=done&assignee=ali&page_id=2&page_size=5",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListTasks(gomock.Any(), gomock.Eq(db.ListTasksParams{
						Status: &done, Assignee: &assignee, Limit: 5, Offset: 5,
					})).
					Times(1).
					Return(tasks[:1], nil)
				store.EXPECT().
					CountTasks(gomock.Any(), gomock.Eq(db.CountTasksParams{
						Status: &done, Assignee: &assignee,
					})).
					Times(1).
					Return(int64(6), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				requireBodyMatchList(t, recorder.Body, listResponse{
					Data: tasks[:1], Total: 6, PageID: 2, PageSize: 5,
				})
			},
		},
		{
			name:  "InvalidStatus",
			query: "?status=blocked",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Times(0)
				store.EXPECT().CountTasks(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "PageSizeTooLarge",
			query: "?page_size=101",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Times(0)
				store.EXPECT().CountTasks(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "NegativePageID",
			query: "?page_id=-1",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Times(0)
				store.EXPECT().CountTasks(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "ListError",
			query: "",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListTasks(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, errors.New("connection refused"))
				store.EXPECT().CountTasks(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
				requireErrorBody(t, recorder.Body, "internal server error")
			},
		},
		{
			name:  "CountError",
			query: "",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListTasks(gomock.Any(), gomock.Any()).
					Times(1).
					Return(tasks, nil)
				store.EXPECT().
					CountTasks(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), errors.New("connection refused"))
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
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

			req, err := http.NewRequest(http.MethodGet, "/api/v1/tasks"+tc.query, nil)
			require.NoError(t, err)

			server.router.ServeHTTP(recorder, req)
			tc.checkResponse(t, recorder)
		})
	}
}

// doRequest sends a request to the server and returns the recorder.
func doRequest(t *testing.T, server *Server, method, url string, body gin.H) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, reader)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, req)
	return recorder
}

// ---------- PATCH /api/v1/tasks/:id ----------

func TestUpdateTaskAPI(t *testing.T) {
	task := randomTask()

	done := db.TaskStatusDone
	updatedStatus := task
	updatedStatus.Status = done

	newTitle := util.RandomTitle()
	newDesc := util.RandomString(40)
	newAssignee := util.RandomAssignee()
	inProgress := db.TaskStatusInProgress
	updatedAll := task
	updatedAll.Title = newTitle
	updatedAll.Description = newDesc
	updatedAll.Status = inProgress
	updatedAll.Assignee = &newAssignee

	testCases := []struct {
		name          string
		id            string
		body          gin.H
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OKOnlyStatus",
			id:   fmt.Sprint(task.ID),
			body: gin.H{"status": "done"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					UpdateTask(gomock.Any(), gomock.Eq(db.UpdateTaskParams{
						ID:     task.ID,
						Status: &done, // all other fields nil → unchanged
					})).
					Times(1).
					Return(updatedStatus, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				requireBodyMatchTask(t, recorder.Body, updatedStatus)
			},
		},
		{
			name: "OKAllFields",
			id:   fmt.Sprint(task.ID),
			body: gin.H{
				"title":       newTitle,
				"description": newDesc,
				"status":      "in_progress",
				"assignee":    newAssignee,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					UpdateTask(gomock.Any(), gomock.Eq(db.UpdateTaskParams{
						ID:          task.ID,
						Title:       &newTitle,
						Description: &newDesc,
						Status:      &inProgress,
						Assignee:    &newAssignee,
					})).
					Times(1).
					Return(updatedAll, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				requireBodyMatchTask(t, recorder.Body, updatedAll)
			},
		},
		{
			name: "NotFound",
			id:   fmt.Sprint(task.ID),
			body: gin.H{"status": "done"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					UpdateTask(gomock.Any(), gomock.Any()).
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
			body: gin.H{"status": "done"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					UpdateTask(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Task{}, errors.New("connection refused"))
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
				requireErrorBody(t, recorder.Body, "internal server error")
			},
		},
		{
			name: "InvalidID",
			id:   "0",
			body: gin.H{"status": "done"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().UpdateTask(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				requireErrorBody(t, recorder.Body, "id must be a positive integer")
			},
		},
		{
			name: "EmptyBody",
			id:   fmt.Sprint(task.ID),
			body: gin.H{},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().UpdateTask(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				requireErrorBody(t, recorder.Body, "at least one field must be provided")
			},
		},
		{
			name: "InvalidStatus",
			id:   fmt.Sprint(task.ID),
			body: gin.H{"status": "blocked"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().UpdateTask(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "EmptyTitle",
			id:   fmt.Sprint(task.ID),
			body: gin.H{"title": ""},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().UpdateTask(gomock.Any(), gomock.Any()).Times(0)
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
			recorder := doRequest(t, server, http.MethodPatch, "/api/v1/tasks/"+tc.id, tc.body)
			tc.checkResponse(t, recorder)
		})
	}
}

// ---------- DELETE /api/v1/tasks/:id ----------

func TestDeleteTaskAPI(t *testing.T) {
	id := util.RandomInt(1, 1000)

	testCases := []struct {
		name          string
		id            string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			id:   fmt.Sprint(id),
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					DeleteTask(gomock.Any(), gomock.Eq(id)).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNoContent, recorder.Code)
				require.Empty(t, recorder.Body.String())
			},
		},
		{
			name: "NotFound",
			id:   fmt.Sprint(id),
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					DeleteTask(gomock.Any(), gomock.Eq(id)).
					Times(1).
					Return(int64(0), nil) // 0 rows affected → didn't exist
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
				requireErrorBody(t, recorder.Body, "task not found")
			},
		},
		{
			name: "InternalError",
			id:   fmt.Sprint(id),
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					DeleteTask(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), errors.New("connection refused"))
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
				requireErrorBody(t, recorder.Body, "internal server error")
			},
		},
		{
			name: "InvalidID",
			id:   "abc",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().DeleteTask(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				requireErrorBody(t, recorder.Body, "id must be a positive integer")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := doRequest(t, server, http.MethodDelete, "/api/v1/tasks/"+tc.id, nil)
			tc.checkResponse(t, recorder)
		})
	}
}
