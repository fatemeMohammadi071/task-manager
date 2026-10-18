package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fatemeMohammadi071/task-manager/internal/api"
	db "github.com/fatemeMohammadi071/task-manager/internal/db/sqlc"
	"github.com/fatemeMohammadi071/task-manager/internal/observability"
	"github.com/fatemeMohammadi071/task-manager/util"
)

// @title			Task Manager API
// @version		1.0
// @description	A small REST service for managing to-do tasks (CRUD, pagination, filtering).
// @host			localhost:8080
// @BasePath		/
// @schemes		http
func main() {
	config, err := util.LoadConfig(".")
	if err != nil {
		log.Fatal("cannot load config: ", err)
	}

	// ctx is cancelled when we receive Ctrl+C (SIGINT) or `docker stop` (SIGTERM)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	shutdownTracer, err := observability.InitTracer(ctx, "task-manager", config.TracingEndpoint)
	if err != nil {
		log.Fatal("cannot init tracer: ", err)
	}
	defer stop()

	poolConfig, err := pgxpool.ParseConfig(config.DBSource)
	if err != nil {
		log.Fatal("cannot parse db config: ", err)
	}
	poolConfig.ConnConfig.Tracer = otelpgx.NewTracer() // one span per SQL query

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Fatal("cannot create db pool: ", err)
	}
	defer pool.Close()

	store := db.NewStore(pool)

	metrics := observability.NewMetrics()
	count, err := store.CountTasks(ctx, db.CountTasksParams{}) // no filters = all tasks

	if err != nil {
		log.Fatal("cannot count tasks: ", err)
	}
	metrics.TasksCount.Set(float64(count))

	if config.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	server := api.NewServer(config, store, metrics)

	httpServer := &http.Server{
		Addr:              config.HTTPServerAddress,
		Handler:           server.Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("HTTP server listening on %s", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("HTTP server error: ", err)
		}
	}()

	<-ctx.Done() // block until a shutdown signal arrives
	log.Println("shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Println("forced shutdown: ", err)
	}

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Println("forced shutdown: ", err)
	}
	if err := shutdownTracer(shutdownCtx); err != nil {
		log.Println("tracer shutdown: ", err)
	}
	log.Println("server stopped")
}
