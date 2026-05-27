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

	tlsEnabled := os.Getenv("TLS_ENABLED") == "true"
	certFile := os.Getenv("CERT_FILE")
	if certFile == "" {
		certFile = "certs/cert.pem"
	}
	keyFile := os.Getenv("KEY_FILE")
	if keyFile == "" {
		keyFile = "certs/key.pem"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "edgeflow-secret-key-change-in-production"
	}

	cfgWatcher, err := controlplane.NewConfigWatcher("config.json")
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	cfg := cfgWatcher.Get()
	if originsEnv := os.Getenv("ORIGINS"); originsEnv != "" {
		cfg.Origins = strings.Split(originsEnv, ",")
	}

	balancer, err := lb.NewWithStrategy(cfg.Origins, lb.ConsistentHash)
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

	jwtMiddleware := security.NewJWTMiddleware(jwtSecret, []string{
		"/health",
		"/metrics",
		"/dashboard",
		"/auth/token",
		"/admin",
	})

	cfgWatcher.OnChange(func(newCfg controlplane.Config) {
		sec.UpdateConfig(security.Config{
			RateLimit:    newCfg.RateLimit,
			MaxBodyBytes: newCfg.MaxBodyBytes,
			Blocklist:    newCfg.Blocklist,
			Allowlist:    newCfg.Allowlist,
		})
		slog.Info("config hot-reloaded", "rate_limit", newCfg.RateLimit)
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
			"hash_ring_size":  balancer.RingSize(),
			"lb_strategy":     "consistent_hash",
			"tls_enabled":     tlsEnabled,
			"config": map[string]any{
				"rate_limit": currentCfg.RateLimit,
				"blocklist":  currentCfg.Blocklist,
			},
		})
	})

	mux.HandleFunc("/auth/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			UserID string `json:"user_id"`
			Role   string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.UserID == "" {
			http.Error(w, "provide user_id and role", http.StatusBadRequest)
			return
		}
		if body.Role == "" {
			body.Role = "user"
		}
		token, err := jwtMiddleware.GenerateToken(body.UserID, body.Role)
		if err != nil {
			http.Error(w, "failed to generate token", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"token":   token,
			"user_id": body.UserID,
			"role":    body.Role,
		})
	})

	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/dashboard", observability.Dashboard())

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
		jwtMiddleware.Handler(
			proxy.CompressionMiddleware(
				sec.Handler(
					cache.Middleware(c, p),
				),
			),
		),
	))

	if tlsEnabled {
		tlsServer := proxy.NewTLSServer(
			":8443",
			":"+port,
			certFile,
			keyFile,
			mux,
		)
		slog.Info("TLS enabled", "cert", certFile, "key", keyFile)
		if err := tlsServer.Start(); err != nil {
			slog.Error("TLS server failed", "error", err)
			os.Exit(1)
		}
	} else {
		slog.Info("EdgeFlow starting", "port", port, "origins", cfg.Origins)
		slog.Info("Dashboard at http://localhost:"+port+"/dashboard")
		slog.Info("To enable TLS: TLS_ENABLED=true go run main.go")
		if err := http.ListenAndServe(":"+port, mux); err != nil {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}
}
