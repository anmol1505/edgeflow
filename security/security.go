package security

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     float64
	capacity float64
}

func NewRateLimiter(rate, capacity float64) *RateLimiter {
	rl := &RateLimiter{
		buckets:  make(map[string]*bucket),
		rate:     rate,
		capacity: capacity,
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	b, ok := rl.buckets[ip]
	if !ok {
		// New IP — start with full capacity minus one token
		rl.buckets[ip] = &bucket{tokens: rl.capacity - 1, lastSeen: time.Now()}
		return true
	}
	now := time.Now()
	elapsed := now.Sub(b.lastSeen).Seconds()
	b.tokens += elapsed * rl.rate
	if b.tokens > rl.capacity {
		b.tokens = rl.capacity
	}
	b.lastSeen = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (rl *RateLimiter) cleanup() {
	for {
		time.Sleep(5 * time.Minute)
		rl.mu.Lock()
		for ip, b := range rl.buckets {
			if time.Since(b.lastSeen) > 10*time.Minute {
				delete(rl.buckets, ip)
			}
		}
		rl.mu.Unlock()
	}
}

type CircuitState int

const (
	StateClosed   CircuitState = iota
	StateOpen
	StateHalfOpen
)

type CircuitBreaker struct {
	mu          sync.Mutex
	state       CircuitState
	failures    int
	successes   int
	lastFailure time.Time
	threshold   int
	timeout     time.Duration
}

func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{threshold: threshold, timeout: timeout, state: StateClosed}
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(cb.lastFailure) > cb.timeout {
			cb.state = StateHalfOpen
			slog.Info("circuit breaker half-open")
			return true
		}
		return false
	case StateHalfOpen:
		return true
	}
	return false
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	cb.successes++
	if cb.state == StateHalfOpen && cb.successes >= 2 {
		cb.state = StateClosed
		cb.successes = 0
		slog.Info("circuit breaker closed")
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	cb.lastFailure = time.Now()
	cb.successes = 0
	if cb.failures >= cb.threshold {
		cb.state = StateOpen
		slog.Warn("circuit breaker open", "failures", cb.failures)
	}
}

func (cb *CircuitBreaker) State() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	}
	return "unknown"
}

type IPFilter struct {
	blocklist map[string]bool
	allowlist map[string]bool
	mu        sync.RWMutex
}

func NewIPFilter(blocklist, allowlist []string) *IPFilter {
	f := &IPFilter{
		blocklist: make(map[string]bool),
		allowlist: make(map[string]bool),
	}
	for _, ip := range blocklist {
		f.blocklist[ip] = true
	}
	for _, ip := range allowlist {
		f.allowlist[ip] = true
	}
	return f
}

func (f *IPFilter) IsAllowed(ip string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.blocklist[ip] {
		return false
	}
	if len(f.allowlist) > 0 && !f.allowlist[ip] {
		return false
	}
	return true
}

type Config struct {
	RateLimit    float64
	MaxBodyBytes int64
	Blocklist    []string
	Allowlist    []string
}

type Middleware struct {
	mu             sync.RWMutex
	limiter        *RateLimiter
	circuitBreaker *CircuitBreaker
	ipFilter       *IPFilter
	maxBodyBytes   int64
}

func New(cfg Config) *Middleware {
	return &Middleware{
		limiter:        NewRateLimiter(cfg.RateLimit, cfg.RateLimit*3),
		circuitBreaker: NewCircuitBreaker(5, 30*time.Second),
		ipFilter:       NewIPFilter(cfg.Blocklist, cfg.Allowlist),
		maxBodyBytes:   cfg.MaxBodyBytes,
	}
}

func getIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getIP(r)

		m.mu.RLock()
		limiter := m.limiter
		ipFilter := m.ipFilter
		maxBodyBytes := m.maxBodyBytes
		m.mu.RUnlock()

		if !ipFilter.IsAllowed(ip) {
			slog.Warn("blocked IP", "ip", ip)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "forbidden"})
			return
		}

		if !limiter.Allow(ip) {
			slog.Warn("rate limited", "ip", ip)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
			return
		}

		if !m.circuitBreaker.Allow() {
			slog.Warn("circuit breaker open")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"error": "service unavailable"})
			return
		}

		if maxBodyBytes > 0 && r.ContentLength > maxBodyBytes {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			json.NewEncoder(w).Encode(map[string]string{"error": "request too large"})
			return
		}
		if maxBodyBytes > 0 {
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		}

		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) CircuitBreaker() *CircuitBreaker {
	return m.circuitBreaker
}

func (m *Middleware) UpdateConfig(cfg Config) {
	newLimiter := NewRateLimiter(cfg.RateLimit, cfg.RateLimit*3)
	newFilter := NewIPFilter(cfg.Blocklist, cfg.Allowlist)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.limiter = newLimiter
	m.ipFilter = newFilter
	m.maxBodyBytes = cfg.MaxBodyBytes
}
