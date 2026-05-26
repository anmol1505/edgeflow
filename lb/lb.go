package lb

import (
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

type Strategy int

const (
	RoundRobin Strategy = iota
	ConsistentHash
)

type Origin struct {
	URL      *url.URL
	Healthy  atomic.Bool
	mu       sync.Mutex
	failures int
}

type LoadBalancer struct {
	origins  []*Origin
	counter  atomic.Uint64
	ring     *hashRing
	strategy Strategy
}

func New(urls []string) (*LoadBalancer, error) {
	return NewWithStrategy(urls, RoundRobin)
}

func NewWithStrategy(urls []string, strategy Strategy) (*LoadBalancer, error) {
	origins := make([]*Origin, 0, len(urls))
	ring := newHashRing(defaultVnodes)

	for _, u := range urls {
		parsed, err := url.Parse(u)
		if err != nil {
			return nil, err
		}
		o := &Origin{URL: parsed}
		o.Healthy.Store(true)
		origins = append(origins, o)
		ring.add(u)
	}

	return &LoadBalancer{
		origins:  origins,
		ring:     ring,
		strategy: strategy,
	}, nil
}

// Next picks next healthy origin using round-robin
func (lb *LoadBalancer) Next() *Origin {
	total := uint64(len(lb.origins))
	for range lb.origins {
		idx := lb.counter.Add(1) % total
		o := lb.origins[idx]
		if o.Healthy.Load() {
			return o
		}
	}
	return nil
}

// NextForKey picks origin using consistent hashing
func (lb *LoadBalancer) NextForKey(key string) *Origin {
	originURL := lb.ring.get(key)
	if originURL == "" {
		return lb.Next()
	}
	// Find the origin object
	for _, o := range lb.origins {
		if o.URL.String() == originURL && o.Healthy.Load() {
			return o
		}
	}
	// Fallback to round-robin if hashed origin is unhealthy
	return lb.Next()
}

// Pick selects origin based on configured strategy
func (lb *LoadBalancer) Pick(r *http.Request) *Origin {
	switch lb.strategy {
	case ConsistentHash:
		// Use client IP + path as hash key for session affinity
		key := r.RemoteAddr + r.URL.Path
		return lb.NextForKey(key)
	default:
		return lb.Next()
	}
}

func (lb *LoadBalancer) MarkFailure(o *Origin) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.failures++
	if o.failures >= 3 {
		o.Healthy.Store(false)
		lb.ring.remove(o.URL.String())
		slog.Warn("origin marked unhealthy, removed from ring", "url", o.URL.String())
	}
}

func (lb *LoadBalancer) MarkSuccess(o *Origin) {
	o.mu.Lock()
	defer o.mu.Unlock()
	wasUnhealthy := !o.Healthy.Load()
	o.failures = 0
	o.Healthy.Store(true)
	if wasUnhealthy {
		lb.ring.add(o.URL.String())
		slog.Info("origin recovered, added back to ring", "url", o.URL.String())
	}
}

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

func (lb *LoadBalancer) HealthyOrigins() []string {
	var result []string
	for _, o := range lb.origins {
		if o.Healthy.Load() {
			result = append(result, o.URL.String())
		}
	}
	return result

}

func (lb *LoadBalancer) RingSize() int {
	return lb.ring.size()
}
