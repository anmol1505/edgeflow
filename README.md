# EdgeFlow

A production-style edge infrastructure platform built in Go — combining reverse proxying, CDN caching, load balancing, security, and observability into a single high-performance system.

> **95,290 requests/second** served from cache with **<1ms p50 latency**

---

## Architecture

```
Client → EdgeFlow Edge Node → Cache Layer → Load Balancer → Origin Servers
                ↓
        Security Layer (rate limit, IP filter, circuit breaker)
                ↓
        Observability (Prometheus metrics, structured logs, request tracing)
                ↓
        Live Admin Dashboard
```

---

## Features

### Reverse Proxy
- HTTP/1.1 request forwarding with connection pooling
- Header rewriting (X-Forwarded-Host, X-Origin, X-Proxied-By)
- Response streaming
- Request ID tracing across all logs

### Load Balancer
- Round-robin across multiple origin servers
- Active health checks every 10 seconds
- Automatic failover on origin failure
- Marks unhealthy after 3 failures, recovers after 2 successes

### Edge Cache
- In-memory LRU cache (configurable max items)
- TTL-based expiry (60s default)
- Stale-while-revalidate: serves stale content while revalidating in background
- ETag / If-None-Match support
- Singleflight request deduplication — prevents cache stampede
- Cache invalidation API by key or prefix
- X-Cache: HIT / MISS / STALE headers

### Security
- Token bucket rate limiter per IP
- Circuit breaker (opens after 5 failures, recovers after 30s)
- IP allowlist / blocklist
- Request body size limits (1MB default)

### Observability
- Prometheus metrics at `/metrics`
- Metrics: request count, cache hit ratio, p50/p95/p99 latency, active connections, rate limited requests
- Structured JSON logs with request ID, method, path, status, duration, cache status
- Live admin dashboard at `/dashboard`

---

## Benchmark Results

| Test | Req/sec | Avg Latency |
|---|---|---|
| Direct Origin | 121,824 | 498µs |
| EdgeFlow (cached) | 95,290 | 585µs |

- **87µs proxy overhead** per request
- **90%+ cache hit rate** under normal usage
- **<1ms p50 latency** for cache hits
- Tested with `wrk` — 4 threads, 50 connections, 10s duration

---

## Quick Start

**Prerequisites:** Go 1.21+

**1. Clone and build:**
```bash
git clone https://github.com/anmol1505/edgeflow.git
cd edgeflow
go build ./...
```

**2. Start origin servers:**
```bash
# Terminal 1
go run benchmarks/origin1/main.go

# Terminal 2
go run benchmarks/origin2/main.go
```

**3. Start EdgeFlow:**
```bash
go run main.go
```

**4. Test it:**
```bash
# Health check
curl http://localhost:8080/health

# Proxy a request (first = MISS, second = HIT)
curl -v http://localhost:8080/hello

# Live dashboard
open http://localhost:8080/dashboard

# Prometheus metrics
curl http://localhost:8080/metrics | grep edgeflow
```

---

## Configuration

| Environment Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | EdgeFlow listen port |
| `ORIGINS` | `http://localhost:9000,http://localhost:9001` | Comma-separated origin URLs |

---

## API Endpoints

| Endpoint | Method | Description |
|---|---|---|
| `/health` | GET | Health status, origin health, cache stats |
| `/metrics` | GET | Prometheus metrics |
| `/dashboard` | GET | Live admin dashboard |
| `/admin/cache/invalidate` | POST | Invalidate cache by key or prefix |

**Cache invalidation example:**
```bash
# By exact key
curl -X POST http://localhost:8080/admin/cache/invalidate \
  -H "Content-Type: application/json" \
  -d '{"key":"GET:/hello"}'

# By prefix
curl -X POST http://localhost:8080/admin/cache/invalidate \
  -H "Content-Type: application/json" \
  -d '{"prefix":"GET:/api"}'
```

---

## Project Structure

```
edgeflow/
├── main.go                 # Entry point, middleware pipeline
├── proxy/
│   └── proxy.go            # HTTP forwarding, header rewriting
├── cache/
│   ├── cache.go            # LRU cache, TTL, eviction
│   └── middleware.go       # Cache middleware, singleflight dedup
├── lb/
│   └── lb.go               # Round-robin LB, health checks, failover
├── security/
│   └── security.go         # Rate limiter, circuit breaker, IP filter
├── observability/
│   ├── metrics.go          # Prometheus metric definitions
│   ├── middleware.go       # Request logging, tracing, metrics
│   └── dashboard.go        # Live admin dashboard
└── benchmarks/
    ├── origin1/main.go     # Test origin server 1
    ├── origin2/main.go     # Test origin server 2
    └── RESULTS.md          # Benchmark results
```

---

## Tech Stack

- **Language:** Go 1.21+
- **Metrics:** Prometheus (`client_golang`)
- **Cache dedup:** `golang.org/x/sync/singleflight`
- **Load testing:** `wrk`

---

## Milestones Completed

- [x] Milestone 1 — Reverse Proxy
- [x] Milestone 2 — Load Balancer + Health Checks
- [x] Milestone 3 — Edge Cache (LRU, TTL, stale-while-revalidate)
- [x] Milestone 4 — Security (rate limiting, circuit breaker, IP filtering)
- [x] Milestone 5 — Prometheus Metrics + Request ID Tracing
- [x] Milestone 6 — Live Admin Dashboard
- [x] Milestone 7 — Benchmarks + Performance Analysis
