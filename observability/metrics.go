package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "edgeflow_requests_total",
		Help: "Total number of requests",
	}, []string{"method", "path", "status"})

	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "edgeflow_request_duration_seconds",
		Help:    "Request duration in seconds",
		Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5},
	}, []string{"method", "path"})

	CacheHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "edgeflow_cache_hits_total",
		Help: "Total cache hits",
	}, []string{"status"}) // HIT, MISS, STALE

	ActiveConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "edgeflow_active_connections",
		Help: "Number of active connections",
	})

	OriginErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "edgeflow_origin_errors_total",
		Help: "Total origin errors",
	}, []string{"origin"})

	RateLimitedRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name: "edgeflow_rate_limited_total",
		Help: "Total rate limited requests",
	})

	CircuitBreakerState = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "edgeflow_circuit_breaker_state",
		Help: "Circuit breaker state: 0=closed, 1=half-open, 2=open",
	}, []string{"origin"})
)
