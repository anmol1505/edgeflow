package lb

import (
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

type Origin struct {
	URL       *url.URL
	Healthy   atomic.Bool
	mu        sync.Mutex
	failures  int
}

type LoadBalancer struct {
	origins []*Origin
	counter atomic.Uint64
}

func New(urls []string) (*LoadBalancer, error) {
	origins := make([]*Origin, 0, len(urls))
	for _, u := range urls {
		parsed, err := url.Parse(u)
		if err != nil {
			return nil, err
		}
		o := &Origin{URL: parsed}
		o.Healthy.Store(true)
		origins = append(origins, o)
	}
	return &LoadBalancer{origins: origins}, nil
}

// Next picks the next healthy origin using round-robin
func (lb *LoadBalancer) Next() *Origin {
	total := uint64(len(lb.origins))
	for range lb.origins {
		idx := lb.counter.Add(1) % total
		o := lb.origins[idx]
		if o.Healthy.Load() {
			return o
		}
	}
	return nil // all origins unhealthy
}

// MarkFailure records a failure and marks unhealthy after 3 failures
func (lb *LoadBalancer) MarkFailure(o *Origin) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.failures++
	if o.failures >= 3 {
		o.Healthy.Store(false)
		slog.Warn("origin marked unhealthy", "url", o.URL.String())
	}
}

// MarkSuccess resets failure count and marks healthy
func (lb *LoadBalancer) MarkSuccess(o *Origin) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.failures = 0
	o.Healthy.Store(true)
}

// StartHealthChecks runs periodic health probes every 10 seconds
func (lb *LoadBalancer) StartHealthChecks() {
	go func() {
		client := &http.Client{Timeout: 2 * time.Second}
		for {
			for _, o := range lb.origins {
				go func(origin *Origin) {
					resp, err := client.Get(origin.URL.String() + "/health")
					if err != nil || resp.StatusCode >= 500 {
						lb.MarkFailure(origin)
					} else {
						lb.MarkSuccess(origin)
						slog.Info("origin healthy", "url", origin.URL.String())
					}
				}(o)
			}
			time.Sleep(10 * time.Second)
		}
	}()
}

// Healthy returns all currently healthy origin URLs (for status reporting)
func (lb *LoadBalancer) HealthyOrigins() []string {
	var result []string
	for _, o := range lb.origins {
		if o.Healthy.Load() {
			result = append(result, o.URL.String())
		}
	}
	return result
}
