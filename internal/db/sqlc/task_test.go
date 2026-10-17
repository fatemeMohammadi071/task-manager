package db

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/fatemeMohammadi071/task-manager/util"
)

// ---------- helpers ----------

func randomStatus() TaskStatus {
	statuses := []TaskStatus{TaskStatusTodo, TaskStatusInProgress, TaskStatusDone}
	return statuses[rand.Intn(len(statuses))]
}

func createTestTask(t *testing.T, assignee string, status TaskStatus) Task {
	t.Helper()

	arg := CreateTaskParams{
		Title:       util.RandomTitle(),
		Description: util.RandomString(30),
		Status:      status,
		Assignee:    &assignee,
	}

	task, err := testStore.CreateTask(context.Background(), arg)
	require.NoError(t, err)
	require.NotZero(t, task.ID)
	require.Equal(t, arg.Title, task.Title)
	require.Equal(t, arg.Description, task.Description)
	require.Equal(t, arg.Status, task.Status)
	require.NotNil(t, task.Assignee)
	require.Equal(t, assignee, *task.Assignee)
	require.NotZero(t, task.CreatedAt)
	require.NotZero(t, task.UpdatedAt)

	return task
}

func createRandomTask(t *testing.T) Task {
	return createTestTask(t, util.RandomAssignee(), randomStatus())
}

// ---------- tests ----------

func TestCreateTask(t *testing.T) {
	createRandomTask(t)
}

func TestGetTask(t *testing.T) {
	task1 := createRandomTask(t)

	task2, err := testStore.GetTask(context.Background(), task1.ID)
	require.NoError(t, err)
	require.Equal(t, task1.ID, task2.ID)
	require.Equal(t, task1.Title, task2.Title)
	require.Equal(t, task1.Description, task2.Description)
	require.Equal(t, task1.Status, task2.Status)
	require.Equal(t, *task1.Assignee, *task2.Assignee)
	require.WithinDuration(t, task1.CreatedAt, task2.CreatedAt, time.Second)
}

func TestGetTaskNotFound(t *testing.T) {
	_, err := testStore.GetTask(context.Background(), -1)
	require.ErrorIs(t, err, pgx.ErrNoRows)
}

func TestUpdateTaskOnlyStatus(t *testing.T) {
	task1 := createTestTask(t, util.RandomAssignee(), TaskStatusTodo)

	newStatus := TaskStatusDone
	task2, err := testStore.UpdateTask(context.Background(), UpdateTaskParams{
		ID:     task1.ID,
		Status: &newStatus, // only status is sent; everything else is nil
	})
	require.NoError(t, err)

	require.Equal(t, TaskStatusDone, task2.Status)
	// fields NOT sent must stay the same (COALESCE)
	require.Equal(t, task1.Title, task2.Title)
	require.Equal(t, task1.Description, task2.Description)
	require.Equal(t, *task1.Assignee, *task2.Assignee)
	require.False(t, task2.UpdatedAt.Before(task1.UpdatedAt))
}

func TestUpdateTaskAllFields(t *testing.T) {
	task1 := createRandomTask(t)

	title := util.RandomTitle()
	desc := util.RandomString(40)
	status := TaskStatusInProgress
	assignee := util.RandomAssignee()

	task2, err := testStore.UpdateTask(context.Background(), UpdateTaskParams{
		ID:          task1.ID,
		Title:       &title,
		Description: &desc,
		Status:      &status,
		Assignee:    &assignee,
	})
	require.NoError(t, err)
	require.Equal(t, title, task2.Title)
	require.Equal(t, desc, task2.Description)
	require.Equal(t, status, task2.Status)
	require.Equal(t, assignee, *task2.Assignee)
}

func TestUpdateTaskNotFound(t *testing.T) {
	title := "x"
	_, err := testStore.UpdateTask(context.Background(), UpdateTaskParams{ID: -1, Title: &title})
	require.ErrorIs(t, err, pgx.ErrNoRows)
}

func TestDeleteTask(t *testing.T) {
	task := createRandomTask(t)

	rows, err := testStore.DeleteTask(context.Background(), task.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), rows)

	_, err = testStore.GetTask(context.Background(), task.ID)
	require.ErrorIs(t, err, pgx.ErrNoRows)

	// deleting again affects 0 rows → handler will return 404
	rows, err = testStore.DeleteTask(context.Background(), task.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), rows)
}

func TestListTasksPagination(t *testing.T) {
	// a unique assignee isolates this test from rows created by other tests
	assignee := util.RandomAssignee()
	for i := 0; i < 5; i++ {
		createTestTask(t, assignee, TaskStatusTodo)
	}

	page1, err := testStore.ListTasks(context.Background(), ListTasksParams{
		Assignee: &assignee, Limit: 3, Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, page1, 3)

	page2, err := testStore.ListTasks(context.Background(), ListTasksParams{
		Assignee: &assignee, Limit: 3, Offset: 3,
	})
	require.NoError(t, err)
	require.Len(t, page2, 2)

	// ORDER BY id → pages must not overlap
	require.Less(t, page1[2].ID, page2[0].ID)
}

func TestListTasksFilterByStatus(t *testing.T) {
	assignee := util.RandomAssignee()
	createTestTask(t, assignee, TaskStatusTodo)
	createTestTask(t, assignee, TaskStatusTodo)
	createTestTask(t, assignee, TaskStatusDone)

	done := TaskStatusDone
	tasks, err := testStore.ListTasks(context.Background(), ListTasksParams{
		Status: &done, Assignee: &assignee, Limit: 10, Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	for _, task := range tasks {
		require.Equal(t, TaskStatusDone, task.Status)
		require.Equal(t, assignee, *task.Assignee)
	}
}

func TestListTasksEmpty(t *testing.T) {
	nobody := util.RandomString(20)
	tasks, err := testStore.ListTasks(context.Background(), ListTasksParams{
		Assignee: &nobody, Limit: 10, Offset: 0,
	})
	require.NoError(t, err)
	require.NotNil(t, tasks) // emit_empty_slices → [] not nil
	require.Empty(t, tasks)
}

func TestCountTasks(t *testing.T) {
	assignee := util.RandomAssignee()
	for i := 0; i < 4; i++ {
		createTestTask(t, assignee, TaskStatusInProgress)
	}

	count, err := testStore.CountTasks(context.Background(), CountTasksParams{Assignee: &assignee})
	require.NoError(t, err)
	require.Equal(t, int64(4), count)

	// no filters → counts the whole table (used later for the tasks_count metric)
	total, err := testStore.CountTasks(context.Background(), CountTasksParams{})
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, int64(4))
}
