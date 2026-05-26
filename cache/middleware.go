package cache

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	defaultTTL  = 60 * time.Second
	staleWindow = 10 * time.Second
)

type responseRecorder struct {
	header     http.Header
	body       bytes.Buffer
	statusCode int
}

func newRecorder() *responseRecorder {
	return &responseRecorder{header: make(http.Header), statusCode: 200}
}

func (r *responseRecorder) Header() http.Header         { return r.header }
func (r *responseRecorder) WriteHeader(code int)        { r.statusCode = code }
func (r *responseRecorder) Write(b []byte) (int, error) { return r.body.Write(b) }

type cachedResponse struct {
	statusCode int
	header     http.Header
	body       []byte
}

var group singleflight.Group

func CacheKey(r *http.Request) string {
	return r.Method + ":" + r.URL.Path
}

func isCacheable(r *http.Request, statusCode int) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	if statusCode < 200 || statusCode >= 300 {
		return false
	}
	cc := r.Header.Get("Cache-Control")
	if strings.Contains(cc, "no-store") || strings.Contains(cc, "no-cache") {
		return false
	}
	return true
}

// safeHeaders copies headers except hop-by-hop ones
func safeHeaders(src http.Header) http.Header {
	dst := make(http.Header)
	skip := map[string]bool{
		"Content-Length":    true,
		"Transfer-Encoding": true,
		"Connection":        true,
		"Keep-Alive":        true,
		"Proxy-Authenticate": true,
		"Proxy-Authorization": true,
		"Te":      true,
		"Trailer": true,
		"Upgrade": true,
	}
	for k, v := range src {
		if !skip[k] {
			dst[k] = v
		}
	}
	return dst
}

func writeEntry(w http.ResponseWriter, entry *Entry, cacheStatus string) {
	w.Header().Set("X-Cache", cacheStatus)
	w.Header().Set("X-Proxied-By", "EdgeFlow")
	if entry.ETag != "" {
		w.Header().Set("ETag", entry.ETag)
	}
	for k, v := range entry.Headers {
		if k != "X-Cache" && k != "X-Proxied-By" {
			w.Header()[k] = v
		}
	}
	w.WriteHeader(entry.StatusCode)
	w.Write(entry.Body)
}

func Middleware(c *Cache, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}

		key := CacheKey(r)

		if entry, ok := c.Get(key); ok {
			if !entry.IsExpired() {
				if r.Header.Get("If-None-Match") == entry.ETag && entry.ETag != "" {
					w.Header().Set("X-Cache", "HIT")
					w.WriteHeader(http.StatusNotModified)
					return
				}
				writeEntry(w, entry, "HIT")
				slog.Debug("cache hit", "key", key)
				return
			}
			if !entry.IsStale() {
				writeEntry(w, entry, "STALE")
				go func() {
					rec := newRecorder()
					next.ServeHTTP(rec, r)
					if isCacheable(r, rec.statusCode) {
						body := make([]byte, rec.body.Len())
						copy(body, rec.body.Bytes())
						c.Set(key, &Entry{
							StatusCode: rec.statusCode,
							Headers:    safeHeaders(rec.header),
							Body:       body,
							ExpiresAt:  time.Now().Add(defaultTTL),
							StaleAt:    time.Now().Add(defaultTTL + staleWindow),
							ETag:       rec.header.Get("ETag"),
						})
					}
				}()
				return
			}
		}

		// Cache MISS with singleflight
		v, err, _ := group.Do(key, func() (interface{}, error) {
			rec := newRecorder()
			next.ServeHTTP(rec, r)
			body := make([]byte, rec.body.Len())
			copy(body, rec.body.Bytes())
			resp := &cachedResponse{
				statusCode: rec.statusCode,
				header:     safeHeaders(rec.header),
				body:       body,
			}
			if isCacheable(r, rec.statusCode) {
				c.Set(key, &Entry{
					StatusCode: rec.statusCode,
					Headers:    safeHeaders(rec.header),
					Body:       body,
					ExpiresAt:  time.Now().Add(defaultTTL),
					StaleAt:    time.Now().Add(defaultTTL + staleWindow),
					ETag:       rec.header.Get("ETag"),
				})
				slog.Debug("cache miss, stored", "key", key)
			}
			return resp, nil
		})

		if err != nil {
			http.Error(w, "origin error", http.StatusBadGateway)
			return
		}

		resp := v.(*cachedResponse)
		w.Header().Set("X-Cache", "MISS")
		w.Header().Set("X-Proxied-By", "EdgeFlow")
		for k, v := range resp.header {
			if k != "X-Cache" && k != "X-Proxied-By" {
				w.Header()[k] = v
			}
		}
		w.WriteHeader(resp.statusCode)
		w.Write(resp.body)
	})
}
