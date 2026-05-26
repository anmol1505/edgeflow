package proxy

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

type Proxy struct {
	target   *url.URL
	reverseProxy *httputil.ReverseProxy
}

func New(targetURL string) (*Proxy, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	rp := httputil.NewSingleHostReverseProxy(target)

	rp.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Set("X-Proxied-By", "EdgeFlow")
		return nil
	}

	return &Proxy{target: target, reverseProxy: rp}, nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Add forwarding headers
	r.Header.Set("X-Forwarded-Host", r.Host)
	r.Header.Set("X-Origin-Host", p.target.Host)

	p.reverseProxy.ServeHTTP(w, r)

	slog.Info("request",
		"method", r.Method,
		"path", r.URL.Path,
		"duration_ms", time.Since(start).Milliseconds(),
		"remote_addr", r.RemoteAddr,
	)
}
