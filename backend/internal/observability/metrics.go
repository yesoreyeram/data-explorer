// Package observability centralizes the three signals every request should
// produce: a structured log line, a Prometheus metric, and a correlation id
// that ties them (and the audit trail entry, if any) together. Keeping this
// in one package means every handler gets the same behavior automatically
// via middleware instead of remembering to instrument itself.
package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	registry         *prometheus.Registry
	httpRequests     *prometheus.CounterVec
	httpDuration     *prometheus.HistogramVec
	dbQueryDuration  *prometheus.HistogramVec
	workflowRuns     *prometheus.CounterVec
	workflowDuration *prometheus.HistogramVec
}

func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewGoCollector())
	reg.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	m := &Metrics{
		registry: reg,
		httpRequests: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Namespace: "data_explorer",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total HTTP requests processed, labeled by route/method/status.",
		}, []string{"route", "method", "status"}),
		httpDuration: promauto.With(reg).NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "data_explorer",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request latency in seconds.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"route", "method"}),
		dbQueryDuration: promauto.With(reg).NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "data_explorer",
			Subsystem: "connector",
			Name:      "query_duration_seconds",
			Help:      "Duration of queries executed against external data sources.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"connection_type", "outcome"}),
		workflowRuns: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Namespace: "data_explorer",
			Subsystem: "workflow",
			Name:      "executions_total",
			Help:      "Total workflow executions, labeled by outcome.",
		}, []string{"outcome"}),
		workflowDuration: promauto.With(reg).NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "data_explorer",
			Subsystem: "workflow",
			Name:      "execution_duration_seconds",
			Help:      "Workflow execution duration in seconds.",
			Buckets:   []float64{.05, .1, .5, 1, 2, 5, 10, 30, 60, 120},
		}, []string{"outcome"}),
	}
	return m
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *Metrics) ObserveHTTP(route, method string, status int, duration time.Duration) {
	m.httpRequests.WithLabelValues(route, method, strconv.Itoa(status)).Inc()
	m.httpDuration.WithLabelValues(route, method).Observe(duration.Seconds())
}

func (m *Metrics) ObserveConnectorQuery(connectionType, outcome string, duration time.Duration) {
	m.dbQueryDuration.WithLabelValues(connectionType, outcome).Observe(duration.Seconds())
}

func (m *Metrics) ObserveWorkflowExecution(outcome string, duration time.Duration) {
	m.workflowRuns.WithLabelValues(outcome).Inc()
	m.workflowDuration.WithLabelValues(outcome).Observe(duration.Seconds())
}
