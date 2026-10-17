package db

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

var testStore Store

func TestMain(m *testing.M) {
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		dbURL = "postgresql://root:secret@localhost:5432/task_manager?sslmode=disable"
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatal("cannot create db pool:", err)
	}
	// pgxpool.New is lazy — Ping makes sure the DB is really reachable.
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatal("cannot connect to db:", err)
	}

	testStore = NewStore(pool)

	code := m.Run()
	pool.Close()
	os.Exit(code)
}
