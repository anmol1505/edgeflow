package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/anmol1505/edgeflow/proxy"
)

func main() {
	origin := os.Getenv("ORIGIN_URL")
	if origin == "" {
		origin = "http://localhost:9000"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	p, err := proxy.New(origin)
	if err != nil {
		slog.Error("failed to create proxy", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","service":"edgeflow"}`))
	})

	// All other requests go to proxy
	mux.Handle("/", p)

	slog.Info("EdgeFlow starting", "port", port, "origin", origin)

	if err := http.ListenAndServe(":"+port, mux); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
