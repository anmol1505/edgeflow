package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"

	controlplane "github.com/anmol1505/edgeflow/control-plane"
	"github.com/anmol1505/edgeflow/cache"
	"github.com/anmol1505/edgeflow/lb"
	"github.com/anmol1505/edgeflow/observability"
	"github.com/anmol1505/edgeflow/proxy"
	"github.com/anmol1505/edgeflow/security"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Load config with hot-reload
	cfgWatcher, err := controlplane.NewConfigWatcher("config.json")
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	cfg := cfgWatcher.Get()

	// Override origins from env if set
	if originsEnv := os.Getenv("ORIGINS"); originsEnv != "" {
		cfg.Origins = strings.Split(originsEnv, ",")
	}

	balancer, err := lb.New(cfg.Origins)
	if err != nil {
		slog.Error("failed to create load balancer", "error", err)
		os.Exit(1)
	}
	balancer.StartHealthChecks()

	c := cache.New(cfg.CacheMaxItems)
	p := proxy.New(balancer)

	sec := security.New(security.Config{
		RateLimit:    cfg.RateLimit,
		MaxBodyBytes: cfg.MaxBodyBytes,
		Blocklist:    cfg.Blocklist,
		Allowlist:    cfg.Allowlist,
	})

	// Hot-reload: update security config when config.json changes
	cfgWatcher.OnChange(func(newCfg controlplane.Config) {
		sec.UpdateConfig(security.Config{
			RateLimit:    newCfg.RateLimit,
			MaxBodyBytes: newCfg.MaxBodyBytes,
			Blocklist:    newCfg.Blocklist,
			Allowlist:    newCfg.Allowlist,
		})
		slog.Info("security config hot-reloaded",
			"rate_limit", newCfg.RateLimit,
			"blocklist", newCfg.Blocklist,
		)
	})

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		currentCfg := cfgWatcher.Get()
		json.NewEncoder(w).Encode(map[string]any{
			"status":          "ok",
			"service":         "edgeflow",
			"healthy_origins": balancer.HealthyOrigins(),
			"cache_stats":     c.Stats(),
			"circuit_breaker": sec.CircuitBreaker().State(),
			"config": map[string]any{
				"rate_limit": currentCfg.RateLimit,
				"blocklist":  currentCfg.Blocklist,
			},
		})
	})

	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/dashboard", observability.Dashboard())

	// Config reload endpoint
	mux.HandleFunc("/admin/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfgWatcher.Get())
	})

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

	mux.Handle("/", observability.Middleware(
		proxy.CompressionMiddleware(
			sec.Handler(
				cache.Middleware(c, p),
			),
		),
	))

	slog.Info("EdgeFlow starting", "port", port, "origins", cfg.Origins)
	slog.Info("Dashboard at http://localhost:" + port + "/dashboard")
	slog.Info("Config hot-reload watching config.json")

	if err := http.ListenAndServe(":"+port, mux); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
