package api

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	db "github.com/fatemeMohammadi071/task-manager/internal/db/sqlc"
)

type createTaskRequest struct {
	Title       string  `json:"title" binding:"required,max=255" example:"Write README"`
	Description string  `json:"description" binding:"max=5000" example:"setup + curl examples"`
	Status      string  `json:"status" binding:"omitempty,oneof=todo in_progress done" example:"todo" enums:"todo,in_progress,done"`
	Assignee    *string `json:"assignee" binding:"omitempty,max=100" example:"fateme"`
}

// createTask godoc
//
//	@Summary		Create a task
//	@Description	Creates a new task. If status is omitted it defaults to "todo".
//	@Tags			tasks
//	@Accept			json
//	@Produce		json
//	@Param			task	body		createTaskRequest	true	"Task to create"
//	@Success		201		{object}	db.Task
//	@Failure		400		{object}	errorBody
//	@Failure		500		{object}	errorBody
//	@Router			/api/v1/tasks [post]
func (server *Server) createTask(ctx *gin.Context) {
	var req createTaskRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	status := db.TaskStatusTodo
	if req.Status != "" {
		status = db.TaskStatus(req.Status)
	}

	arg := db.CreateTaskParams{
		Title:       req.Title,
		Description: req.Description,
		Status:      status,
		Assignee:    req.Assignee,
	}

	task, err := server.store.CreateTask(ctx, arg)
	if err != nil {
		server.internalError(ctx, err)
		return
	}

	server.metrics.TasksCount.Inc()
	ctx.JSON(http.StatusCreated, task)
}

// taskIDRequest is shared by GET, PATCH and DELETE /tasks/:id
type taskIDRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// bindTaskID reads :id from the URL. On failure it writes a 400 and returns false.
func bindTaskID(ctx *gin.Context) (int64, bool) {
	var req taskIDRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "id must be a positive integer"})
		return 0, false
	}
	return req.ID, true
}

// getTask godoc
//
//	@Summary	Get a task by ID
//	@Tags		tasks
//	@Produce	json
//	@Param		id	path		int	true	"Task ID"	minimum(1)
//	@Success	200	{object}	db.Task
//	@Failure	400	{object}	errorBody
//	@Failure	404	{object}	errorBody
//	@Failure	500	{object}	errorBody
//	@Router		/api/v1/tasks/{id} [get]
func (server *Server) getTask(ctx *gin.Context) {
	id, ok := bindTaskID(ctx)
	if !ok {
		return
	}

	task, err := server.store.GetTask(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		server.internalError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, task)
}

// internalError logs the real error and returns a generic message,
// so DB details are never leaked to clients.
func (server *Server) internalError(ctx *gin.Context, err error) {
	log.Printf("internal error on %s %s: %v", ctx.Request.Method, ctx.FullPath(), err)
	ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}

const (
	defaultPageID   = 1
	defaultPageSize = 10
)

type listTasksRequest struct {
	// max on page_id prevents int32 overflow in the offset calculation
	PageID   int32  `form:"page_id" binding:"omitempty,min=1,max=100000"`
	PageSize int32  `form:"page_size" binding:"omitempty,min=1,max=100"`
	Status   string `form:"status" binding:"omitempty,oneof=todo in_progress done"`
	Assignee string `form:"assignee" binding:"omitempty,max=100"`
}

type listTasksResponse struct {
	Data     []db.Task `json:"data"`
	Total    int64     `json:"total"`
	PageID   int32     `json:"page_id"`
	PageSize int32     `json:"page_size"`
}

// listTasks godoc
//
//	@Summary		List tasks
//	@Description	Paginated list with optional filters. Response includes total count.
//	@Tags			tasks
//	@Produce		json
//	@Param			page_id		query		int		false	"Page number (default 1)"			minimum(1)
//	@Param			page_size	query		int		false	"Page size (default 10, max 100)"	minimum(1)	maximum(100)
//	@Param			status		query		string	false	"Filter by status"					Enums(todo, in_progress, done)
//	@Param			assignee	query		string	false	"Filter by assignee"
//	@Success		200			{object}	listTasksResponse
//	@Failure		400			{object}	errorBody
//	@Failure		500			{object}	errorBody
//	@Router			/api/v1/tasks [get]
func (server *Server) listTasks(ctx *gin.Context) {
	var req listTasksRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if req.PageID == 0 {
		req.PageID = defaultPageID
	}
	if req.PageSize == 0 {
		req.PageSize = defaultPageSize
	}

	// empty query param → nil pointer → "no filter" in SQL
	var status *db.TaskStatus
	if req.Status != "" {
		s := db.TaskStatus(req.Status)
		status = &s
	}
	var assignee *string
	if req.Assignee != "" {
		assignee = &req.Assignee
	}

	tasks, err := server.store.ListTasks(ctx, db.ListTasksParams{
		Status:   status,
		Assignee: assignee,
		Limit:    req.PageSize,
		Offset:   (req.PageID - 1) * req.PageSize,
	})
	if err != nil {
		server.internalError(ctx, err)
		return
	}

	total, err := server.store.CountTasks(ctx, db.CountTasksParams{
		Status:   status,
		Assignee: assignee,
	})
	if err != nil {
		server.internalError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, listTasksResponse{
		Data:     tasks,
		Total:    total,
		PageID:   req.PageID,
		PageSize: req.PageSize,
	})
}

type updateTaskRequest struct {
	// pointers: nil = "not sent" = keep the old value (COALESCE in SQL)
	Title       *string `json:"title" binding:"omitempty,min=1,max=255"`
	Description *string `json:"description" binding:"omitempty,max=5000"`
	Status      *string `json:"status" binding:"omitempty,oneof=todo in_progress done"`
	Assignee    *string `json:"assignee" binding:"omitempty,max=100"`
}

// updateTask godoc
//
//	@Summary		Partially update a task
//	@Description	Only the fields sent are changed. At least one field is required.
//	@Tags			tasks
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int					true	"Task ID"	minimum(1)
//	@Param			task	body		updateTaskRequest	true	"Fields to update"
//	@Success		200		{object}	db.Task
//	@Failure		400		{object}	errorBody
//	@Failure		404		{object}	errorBody
//	@Failure		500		{object}	errorBody
//	@Router			/api/v1/tasks/{id} [patch]
func (server *Server) updateTask(ctx *gin.Context) {
	id, ok := bindTaskID(ctx)
	if !ok {
		return
	}

	var req updateTaskRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if req.Title == nil && req.Description == nil && req.Status == nil && req.Assignee == nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "at least one field must be provided"})
		return
	}

	var status *db.TaskStatus
	if req.Status != nil {
		s := db.TaskStatus(*req.Status)
		status = &s
	}

	task, err := server.store.UpdateTask(ctx, db.UpdateTaskParams{
		ID:          id,
		Title:       req.Title,
		Description: req.Description,
		Status:      status,
		Assignee:    req.Assignee,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		server.internalError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, task)
}

// deleteTask godoc
//
//	@Summary	Delete a task
//	@Tags		tasks
//	@Param		id	path	int	true	"Task ID"	minimum(1)
//	@Success	204	"No Content"
//	@Failure	400	{object}	errorBody
//	@Failure	404	{object}	errorBody
//	@Failure	500	{object}	errorBody
//	@Router		/api/v1/tasks/{id} [delete]
func (server *Server) deleteTask(ctx *gin.Context) {
	id, ok := bindTaskID(ctx)
	if !ok {
		return
	}

	rows, err := server.store.DeleteTask(ctx, id)
	if err != nil {
		server.internalError(ctx, err)
		return
	}
	if rows == 0 {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	server.metrics.TasksCount.Dec()
	ctx.Status(http.StatusNoContent)
}
