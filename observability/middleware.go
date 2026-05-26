package observability

import (
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func generateRequestID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), rand.Intn(9999))
}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Request ID tracing
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		w.Header().Set("X-Request-ID", requestID)

		// Track active connections
		ActiveConnections.Inc()
		defer ActiveConnections.Dec()

		// Wrap writer to capture status code
		rec := &statusRecorder{ResponseWriter: w, statusCode: 200}

		next.ServeHTTP(rec, r)

		duration := time.Since(start)
		status := strconv.Itoa(rec.statusCode)

		// Normalize path to avoid high cardinality
		path := r.URL.Path
		if len(path) > 50 {
			path = path[:50]
		}

		// Record metrics
		RequestsTotal.WithLabelValues(r.Method, path, status).Inc()
		RequestDuration.WithLabelValues(r.Method, path).Observe(duration.Seconds())

		// Track cache status
		cacheStatus := w.Header().Get("X-Cache")
		if cacheStatus != "" {
			CacheHits.WithLabelValues(cacheStatus).Inc()
		}

		// Track rate limiting
		if rec.statusCode == http.StatusTooManyRequests {
			RateLimitedRequests.Inc()
		}

		// Structured log with request ID
		slog.Info("request completed",
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.statusCode,
			"duration_ms", duration.Milliseconds(),
			"cache", cacheStatus,
			"remote_addr", r.RemoteAddr,
		)
	})
}
