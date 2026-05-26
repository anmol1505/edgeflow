package proxy

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/anmol1505/edgeflow/lb"
)

type Proxy struct {
	lb *lb.LoadBalancer
}

func New(loadBalancer *lb.LoadBalancer) *Proxy {
	return &Proxy{lb: loadBalancer}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	origin := p.lb.Pick(r)
	if origin == nil {
		http.Error(w, "no healthy origins available", http.StatusBadGateway)
		slog.Error("all origins unhealthy")
		return
	}

	rp := httputil.NewSingleHostReverseProxy(origin.URL)

	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		p.lb.MarkFailure(origin)
		slog.Error("origin request failed", "url", origin.URL.String(), "error", err)
		http.Error(w, "origin error", http.StatusBadGateway)
	}

	rp.ModifyResponse = func(resp *http.Response) error {
		p.lb.MarkSuccess(origin)
		resp.Header.Set("X-Proxied-By", "EdgeFlow")
		resp.Header.Set("X-Origin", origin.URL.Host)
		return nil
	}

	r.Header.Set("X-Forwarded-Host", r.Host)
	r.Header.Set("X-Origin-Host", origin.URL.Host)

	rp.ServeHTTP(w, r)

	slog.Info("request",
		"method", r.Method,
		"path", r.URL.Path,
		"origin", origin.URL.Host,
		"duration_ms", time.Since(start).Milliseconds(),
	)
}
