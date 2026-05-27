package proxy

import (
	"crypto/tls"
	"log/slog"
	"net/http"
	"time"
)

type TLSServer struct {
	HTTPSAddr string
	HTTPAddr  string
	CertFile  string
	KeyFile   string
	Handler   http.Handler
}

func NewTLSServer(httpsAddr, httpAddr, certFile, keyFile string, handler http.Handler) *TLSServer {
	return &TLSServer{
		HTTPSAddr: httpsAddr,
		HTTPAddr:  httpAddr,
		CertFile:  certFile,
		KeyFile:   keyFile,
		Handler:   handler,
	}
}

func (s *TLSServer) Start() error {
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}

	httpsServer := &http.Server{
		Addr:         s.HTTPSAddr,
		Handler:      s.Handler,
		TLSConfig:    tlsCfg,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// HTTP server redirects to HTTPS
	httpServer := &http.Server{
		Addr: s.HTTPAddr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			target := "https://localhost:8443" + r.URL.RequestURI()
			http.Redirect(w, r, target, http.StatusMovedPermanently)
		}),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("HTTP redirect server starting", "addr", s.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
		}
	}()

	slog.Info("HTTPS server starting", "addr", s.HTTPSAddr)
	return httpsServer.ListenAndServeTLS(s.CertFile, s.KeyFile)
}
