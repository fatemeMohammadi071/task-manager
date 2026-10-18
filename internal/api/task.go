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
	Title       string  `json:"title" binding:"required,max=255"`
	Description string  `json:"description" binding:"max=5000"`
	Status      string  `json:"status" binding:"omitempty,oneof=todo in_progress done"`
	Assignee    *string `json:"assignee" binding:"omitempty,max=100"`
}

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

	ctx.JSON(http.StatusCreated, task)
}

// taskIDRequest is shared by GET, PATCH and DELETE /tasks/:id
type taskIDRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

func (server *Server) getTask(ctx *gin.Context) {
	var req taskIDRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	task, err := server.store.GetTask(ctx, req.ID)
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
