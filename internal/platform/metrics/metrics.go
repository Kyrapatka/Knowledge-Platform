package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics is owned by one App, never the global registry.
type Metrics struct {
	Registry    *prometheus.Registry
	Requests    *prometheus.CounterVec
	Duration    *prometheus.HistogramVec
	InFlight    prometheus.Gauge
	Published   prometheus.Counter
	Dropped     *prometheus.CounterVec
	Flushes     prometheus.Counter
	FlushErrors prometheus.Counter
}

func New() *Metrics {
	m := &Metrics{
		Registry:    prometheus.NewRegistry(),
		Requests:    prometheus.NewCounterVec(prometheus.CounterOpts{Name: "http_requests_total", Help: "Completed HTTP requests."}, []string{"method", "route", "status"}),
		Duration:    prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "http_request_duration_seconds", Help: "HTTP handler duration in seconds.", Buckets: prometheus.DefBuckets}, []string{"method", "route", "status"}),
		InFlight:    prometheus.NewGauge(prometheus.GaugeOpts{Name: "http_requests_in_flight", Help: "Active HTTP handlers excluding metrics scrapes."}),
		Published:   prometheus.NewCounter(prometheus.CounterOpts{Name: "analytics_events_published_total", Help: "Events accepted into the in-memory queue, not delivered acknowledgements."}),
		Dropped:     prometheus.NewCounterVec(prometheus.CounterOpts{Name: "analytics_events_dropped_total", Help: "Events lost by bounded delivery."}, []string{"reason"}),
		Flushes:     prometheus.NewCounter(prometheus.CounterOpts{Name: "analytics_batch_flush_total", Help: "Attempted nonempty batch inserts."}),
		FlushErrors: prometheus.NewCounter(prometheus.CounterOpts{Name: "analytics_batch_flush_errors_total", Help: "Failed or ambiguous batch inserts."}),
	}
	m.Registry.MustRegister(m.Requests, m.Duration, m.InFlight, m.Published, m.Dropped, m.Flushes, m.FlushErrors, collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	for _, reason := range []string{"buffer_full", "closed", "flush_error", "shutdown_timeout", "invalid"} {
		m.Dropped.WithLabelValues(reason)
	}
	return m
}
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{})
}
func (m *Metrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.FullPath() == "/metrics" {
			c.Next()
			return
		}
		start := time.Now()
		m.InFlight.Inc()
		defer func() {
			m.InFlight.Dec()
			route := c.FullPath()
			if route == "" {
				route = "unmatched"
			}
			method := c.Request.Method
			switch method {
			case "GET", "HEAD", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "CONNECT", "TRACE":
			default:
				method = "OTHER"
			}
			status := c.Writer.Status()
			if status < 100 || status > 599 {
				status = 0
			}
			labels := []string{method, route, strconv.Itoa(status)}
			m.Requests.WithLabelValues(labels...).Inc()
			m.Duration.WithLabelValues(labels...).Observe(time.Since(start).Seconds())
		}()
		c.Next()
	}
}
