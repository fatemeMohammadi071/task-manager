package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	db "github.com/fatemeMohammadi071/task-manager/internal/db/sqlc"
	"github.com/fatemeMohammadi071/task-manager/util"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/fatemeMohammadi071/task-manager/docs"
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
	router.Use(
		gin.LoggerWithConfig(gin.LoggerConfig{SkipPaths: []string{"/health", "/metrics"}}),
		gin.Recovery(),
	)

	router.GET("/health", server.health)
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	v1 := router.Group("/api/v1")
	{
		v1.POST("/tasks", server.createTask)
		v1.GET("/tasks", server.listTasks)
		v1.GET("/tasks/:id", server.getTask)
		v1.PATCH("/tasks/:id", server.updateTask)
		v1.DELETE("/tasks/:id", server.deleteTask)
	}

	server.router = router
}

// Router exposes the handler so main.go can run it inside an http.Server.
func (server *Server) Router() http.Handler {
	return server.router
}

// health godoc
//
//	@Summary	Health check
//	@Tags		health
//	@Produce	json
//	@Success	200	{object}	map[string]string
//	@Router		/health [get]
func (server *Server) health(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func errorResponse(err error) gin.H {
	return gin.H{"error": err.Error()}
}

// errorBody is the JSON shape of every error response.
type errorBody struct {
	Error string `json:"error" example:"task not found"`
}
