package proxy

import (
	"compress/gzip"
	"net/http"
	"strings"

	"github.com/andybalholm/brotli"
)

type gzipResponseWriter struct {
	http.ResponseWriter
	gz *gzip.Writer
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	return g.gz.Write(b)
}

func (g *gzipResponseWriter) WriteHeader(code int) {
	g.ResponseWriter.WriteHeader(code)
}

type brotliResponseWriter struct {
	http.ResponseWriter
	br *brotli.Writer
}

func (b *brotliResponseWriter) Write(data []byte) (int, error) {
	return b.br.Write(data)
}

func (b *brotliResponseWriter) WriteHeader(code int) {
	b.ResponseWriter.WriteHeader(code)
}

func bestEncoding(r *http.Request) string {
	accept := r.Header.Get("Accept-Encoding")
	if strings.Contains(accept, "br") {
		return "br"
	}
	if strings.Contains(accept, "gzip") {
		return "gzip"
	}
	return ""
}

// CompressionMiddleware compresses responses using brotli or gzip
func CompressionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encoding := bestEncoding(r)
		if encoding == "" {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Del("Content-Length")
		w.Header().Add("Vary", "Accept-Encoding")

		switch encoding {
		case "br":
			w.Header().Set("Content-Encoding", "br")
			bw := brotli.NewWriterLevel(w, brotli.BestSpeed)
			defer bw.Close()
			next.ServeHTTP(&brotliResponseWriter{ResponseWriter: w, br: bw}, r)
		case "gzip":
			w.Header().Set("Content-Encoding", "gzip")
			gz, _ := gzip.NewWriterLevel(w, gzip.BestSpeed)
			defer gz.Close()
			next.ServeHTTP(&gzipResponseWriter{ResponseWriter: w, gz: gz}, r)
		}
	})
}
