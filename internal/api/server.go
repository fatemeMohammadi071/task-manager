package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	db "github.com/fatemeMohammadi071/task-manager/internal/db/sqlc"
	"github.com/fatemeMohammadi071/task-manager/util"
)

// Server serves HTTP requests for the task manager service.
type Server struct {
	config util.Config
	store  db.Store
	router *gin.Engine
}

// NewServer creates a new HTTP server and sets up routing.
func NewServer(config util.Config, store db.Store) *Server {
	server := &Server{
		config: config,
		store:  store,
	}
	server.setupRouter()
	return server
}

func (server *Server) setupRouter() {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	router.GET("/health", server.health)

		v1 := router.Group("/api/v1")
	{
		v1.POST("/tasks", server.createTask)
	}

	server.router = router
}

// Router exposes the handler so main.go can run it inside an http.Server.
func (server *Server) Router() http.Handler {
	return server.router
}

func (server *Server) health(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func errorResponse(err error) gin.H {
	return gin.H{"error": err.Error()}
}
