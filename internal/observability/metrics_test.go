package observability

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func newTestRouter(m *Metrics) *gin.Engine {
	r := gin.New()
	r.Use(m.Middleware(), gin.Recovery())
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })
	r.GET("/items/:id", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/boom", func(c *gin.Context) { panic("boom") })
	return r
}

func send(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestRequestsTotal(t *testing.T) {
	m := NewMetrics()
	r := newTestRouter(m)

	send(r, "/ping")
	send(r, "/ping")

	require.Equal(t, 2.0, testutil.ToFloat64(m.RequestsTotal.WithLabelValues("GET", "/ping", "200")))
}

func TestPathLabelUsesRouteTemplate(t *testing.T) {
	m := NewMetrics()
	r := newTestRouter(m)

	send(r, "/items/1")
	send(r, "/items/2")

	// one series for "/items/:id", NOT one per id
	require.Equal(t, 1, testutil.CollectAndCount(m.RequestsTotal))
	require.Equal(t, 2.0, testutil.ToFloat64(m.RequestsTotal.WithLabelValues("GET", "/items/:id", "200")))
}

func TestUnmatchedRouteLabel(t *testing.T) {
	m := NewMetrics()
	r := newTestRouter(m)

	send(r, "/does-not-exist")

	require.Equal(t, 1.0, testutil.ToFloat64(m.RequestsTotal.WithLabelValues("GET", "unmatched", "404")))
}

func TestPanicCountedAs500(t *testing.T) {
	m := NewMetrics()
	r := newTestRouter(m)

	rec := send(r, "/boom")

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Equal(t, 1.0, testutil.ToFloat64(m.RequestsTotal.WithLabelValues("GET", "/boom", "500")))
}

func TestLatencyObserved(t *testing.T) {
	m := NewMetrics()
	r := newTestRouter(m)

	send(r, "/ping")

	require.Equal(t, 1, testutil.CollectAndCount(m.RequestLatency))
}

func TestNewMetricsTwiceDoesNotPanic(t *testing.T) {
	require.NotPanics(t, func() {
		NewMetrics()
		NewMetrics()
	})
}

func TestHandlerExposesMetrics(t *testing.T) {
	m := NewMetrics()
	m.TasksCount.Set(3)

	rec := send(m.Handler(), "/metrics")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "tasks_count 3")
	require.Contains(t, rec.Body.String(), "go_goroutines") // runtime metrics included
}
