package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds the Prometheus collectors for the service.
// Each instance has its own registry, so tests can create many without conflicts.
type Metrics struct {
	registry       *prometheus.Registry
	RequestsTotal  *prometheus.CounterVec
	RequestLatency *prometheus.HistogramVec
	TasksCount     prometheus.Gauge
}

func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()

	m := &Metrics{
		registry: reg,
		RequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "requests_total",
			Help: "Total number of HTTP requests.",
		}, []string{"method", "path", "status"}),
		RequestLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "request_latency_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"}),
		TasksCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "tasks_count",
			Help: "Current number of tasks.",
		}),
	}

	reg.MustRegister(
		m.RequestsTotal,
		m.RequestLatency,
		m.TasksCount,
		collectors.NewGoCollector(), // goroutines, GC, memory
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}), // CPU, open files
	)
	return m
}

// Handler serves the /metrics endpoint.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// Middleware records requests_total and request_latency_seconds for every request.
func (m *Metrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next() // run the rest of the chain (handler etc.)

		path := c.FullPath() // route template, e.g. "/api/v1/tasks/:id"
		if path == "" {
			path = "unmatched" // 404s: don't put raw URLs into labels
		}
		status := strconv.Itoa(c.Writer.Status())

		m.RequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		m.RequestLatency.WithLabelValues(c.Request.Method, path).Observe(time.Since(start).Seconds())
	}
}
