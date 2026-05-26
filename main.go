package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/anmol1505/edgeflow/lb"
	"github.com/anmol1505/edgeflow/proxy"
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

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		healthy := balancer.HealthyOrigins()
		json.NewEncoder(w).Encode(map[string]any{
			"status":          "ok",
			"service":         "edgeflow",
			"healthy_origins": healthy,
		})
	})

	mux.Handle("/", p)

	slog.Info("EdgeFlow starting", "port", port, "origins", originList)

	if err := http.ListenAndServe(":"+port, mux); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
