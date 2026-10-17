package api

import (
	"os"
	"testing"

	"github.com/gin-gonic/gin"

	db "github.com/fatemeMohammadi071/task-manager/internal/db/sqlc"
	"github.com/fatemeMohammadi071/task-manager/util"
)

func newTestServer(t *testing.T, store db.Store) *Server {
	t.Helper()
	config := util.Config{Environment: "test"}
	return NewServer(config, store)
}

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode) // less noisy logs in tests
	os.Exit(m.Run())
}
