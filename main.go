package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/anmol1505/edgeflow/cache"
	"github.com/anmol1505/edgeflow/lb"
	"github.com/anmol1505/edgeflow/observability"
	"github.com/anmol1505/edgeflow/proxy"
	"github.com/anmol1505/edgeflow/security"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	originsEnv := os.Getenv("ORIGINS")
	if originsEnv == "" {
		originsEnv = "http://localhost:9000,http://localhost:9001"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	originList := strings.Split(originsEnv, ",")

	balancer, err := lb.New(originList)
	if err != nil {
		slog.Error("failed to create load balancer", "error", err)
		os.Exit(1)
	}
	balancer.StartHealthChecks()

	p := proxy.New(balancer)
	c := cache.New(1000)

	sec := security.New(security.Config{
		RateLimit:    3,
		MaxBodyBytes: 1 << 20,
		Blocklist:    []string{},
		Allowlist:    []string{},
	})

	mux := http.NewServeMux()

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":          "ok",
			"service":         "edgeflow",
			"healthy_origins": balancer.HealthyOrigins(),
			"cache_stats":     c.Stats(),
			"circuit_breaker": sec.CircuitBreaker().State(),
		})
	})

	// Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Cache invalidation API
	mux.HandleFunc("/admin/cache/invalidate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Key    string `json:"key"`
			Prefix string `json:"prefix"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if body.Key != "" {
			c.Delete(body.Key)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"invalidated": body.Key})
			return
		}
		if body.Prefix != "" {
			count := c.InvalidatePrefix(body.Prefix)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"invalidated_count": count})
			return
		}
		http.Error(w, "provide key or prefix", http.StatusBadRequest)
	})

	// Full pipeline: Observability -> Security -> Cache -> Proxy
	mux.Handle("/", observability.Middleware(sec.Handler(cache.Middleware(c, p))))

	slog.Info("EdgeFlow starting", "port", port, "origins", originList)

	if err := http.ListenAndServe(":"+port, mux); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
