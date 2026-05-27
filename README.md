# EdgeFlow

A production-style edge infrastructure platform built in Go — combining reverse proxying, CDN caching, load balancing, security, and observability into a single high-performance system.

> **95,290 requests/second** served from cache · **<1ms p50 latency** · **13 production features**

---

## Architecture

```
Client → EdgeFlow Edge Node
            ↓
    [OpenTelemetry Tracing]
            ↓
    [JWT Authentication]
            ↓
    [Gzip/Brotli Compression]
            ↓
    [Token Bucket Rate Limiter]
    [Circuit Breaker]
    [IP Filter]
            ↓
    [LRU Cache + Stale-While-Revalidate]
    [Singleflight Dedup]
            ↓
    [Consistent Hash Ring]
    [Round Robin LB]
    [Health Checks + Failover]
            ↓
    Origin Servers (1..N)
```

---

## Features

### Reverse Proxy
- HTTP/1.1 request forwarding with connection pooling
- Header rewriting (X-Forwarded-Host, X-Origin, X-Proxied-By)
- Response streaming
- Request ID tracing across all logs
- **WebSocket support** with HTTP upgrade and bidirectional streaming

### Load Balancer
- Round-robin across multiple origin servers
- **Consistent hashing ring** with 150 virtual nodes per origin
- Session affinity — same client always routes to same origin
- Active health checks every 10 seconds
- Automatic failover — unhealthy origins removed from ring
- Auto-recovery when origins come back

### Edge Cache
- In-memory LRU cache (configurable max items)
- TTL-based expiry (60s default)
- **Stale-while-revalidate** — serves stale while revalidating in background
- ETag / If-None-Match support
- **Singleflight request deduplication** — prevents cache stampede
- Cache invalidation API by exact key or prefix
- X-Cache: HIT / MISS / STALE headers

### Security
- **Token bucket rate limiter** per IP (configurable req/sec)
- **Circuit breaker** — opens after 5 failures, recovers after 30s
- IP allowlist / blocklist
- Request body size limits
- **JWT authentication** with HS256 signing
- Role-based claims forwarded to origins (X-User-ID, X-User-Role)
- Public path exclusions (health, metrics, dashboard)

### Compression
- **Brotli** compression (preferred)
- **Gzip** compression (fallback)
- Automatic encoding negotiation via Accept-Encoding
- Skips already-compressed content types

### TLS
- **TLS termination** with TLS 1.2+ minimum
- Strong cipher suites (AES-256-GCM, AES-128-GCM)
- HTTP → HTTPS automatic redirect (301)
- Configurable cert/key paths

### Config Hot-Reload
- Watches `config.json` for changes every 2 seconds
- **Zero downtime** config updates
- Hot-reloads: rate limits, blocklist, allowlist, body size limits
- No restart required

### Observability
- **Prometheus metrics** at `/metrics`
- **OpenTelemetry distributed tracing** with span attributes
- Trace context propagation across proxy → origin
- Structured JSON logs with request ID, method, path, status, duration
- **Live admin dashboard** at `/dashboard` (updates every 2s)
- **Grafana dashboard** with Prometheus datasource

### Metrics Tracked
- Request count by method, path, status
- Cache hit ratio (HIT/MISS/STALE)
- p50/p95/p99 latency histograms
- Active connections gauge
- Rate limited request counter
- Origin error rate

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
go run benchmarks/origin1/main.go   # Terminal 1
go run benchmarks/origin2/main.go   # Terminal 2
```

**3. Start EdgeFlow:**
```bash
go run main.go
```

**4. Test it:**
```bash
# Health check
curl http://localhost:8080/health

# Get JWT token
curl -X POST http://localhost:8080/auth/token \
  -H "Content-Type: application/json" \
  -d '{"user_id": "alice", "role": "admin"}'

# Authenticated request (first=MISS, second=HIT)
TOKEN="your_token_here"
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/hello

# WebSocket
wscat -c ws://localhost:8080/ws

# Live dashboard
open http://localhost:8080/dashboard

# Prometheus metrics
curl http://localhost:8080/metrics | grep edgeflow
```

**5. Enable TLS:**
```bash
TLS_ENABLED=true go run main.go
curl -k https://localhost:8443/health
```

---

## Configuration

`config.json` is watched and hot-reloaded every 2 seconds:

```json
{
  "rate_limit": 100,
  "max_body_bytes": 1048576,
  "blocklist": ["1.2.3.4"],
  "allowlist": [],
  "origins": ["http://localhost:9000", "http://localhost:9001"],
  "cache_max_items": 1000,
  "cache_ttl_secs": 60
}
```

| Environment Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Listen port |
| `TLS_ENABLED` | `false` | Enable HTTPS |
| `CERT_FILE` | `certs/cert.pem` | TLS certificate |
| `KEY_FILE` | `certs/key.pem` | TLS private key |
| `JWT_SECRET` | `edgeflow-secret-key` | JWT signing secret |
| `ORIGINS` | `localhost:9000,9001` | Override origins |

---

## API Endpoints

| Endpoint | Method | Auth | Description |
|---|---|---|---|
| `/health` | GET | No | Health + origin status + cache stats |
| `/metrics` | GET | No | Prometheus metrics |
| `/dashboard` | GET | No | Live admin dashboard |
| `/auth/token` | POST | No | Generate JWT token |
| `/admin/config` | GET | No | View current config |
| `/admin/cache/invalidate` | POST | No | Invalidate cache |
| `/ws` | WS | No | WebSocket proxy |
| `/*` | ANY | JWT | Proxied to origins |

---

## Project Structure

```
edgeflow/
├── main.go                     # Entry point, middleware pipeline
├── config.json                 # Hot-reloadable config
├── certs/                      # TLS certificates
├── proxy/
│   ├── proxy.go                # HTTP forwarding, header rewriting
│   ├── compression.go          # Gzip/Brotli middleware
│   ├── tls.go                  # TLS termination
│   └── websocket.go            # WebSocket upgrade proxying
├── cache/
│   ├── cache.go                # LRU cache, TTL, eviction
│   └── middleware.go           # Cache middleware, singleflight
├── lb/
│   ├── lb.go                   # Load balancer, health checks
│   └── consistent_hash.go      # Consistent hash ring
├── security/
│   ├── security.go             # Rate limiter, circuit breaker, IP filter
│   └── jwt.go                  # JWT middleware
├── observability/
│   ├── metrics.go              # Prometheus metrics
│   ├── middleware.go           # Request logging, tracing
│   ├── tracing.go              # OpenTelemetry setup
│   └── dashboard.go            # Live admin dashboard
├── control-plane/
│   └── config.go               # Config hot-reload watcher
└── benchmarks/
    ├── origin1/main.go         # Test origin 1
    ├── origin2/main.go         # Test origin 2
    ├── ws_server/main.go       # WebSocket test server
    ├── prometheus.yml          # Prometheus config
    └── RESULTS.md              # Benchmark results
```

---

## Tech Stack

- **Language:** Go 1.21+
- **Metrics:** Prometheus (`client_golang`)
- **Tracing:** OpenTelemetry SDK
- **Auth:** JWT (`golang-jwt/jwt`)
- **Compression:** Brotli (`andybalholm/brotli`), stdlib gzip
- **Cache dedup:** `golang.org/x/sync/singleflight`
- **Dashboards:** Grafana + Prometheus
- **Load testing:** `wrk`

---

## Middleware Pipeline

```
Request
  → OpenTelemetry Trace Span
  → Prometheus Metrics + Structured Logging
  → JWT Authentication
  → Gzip/Brotli Compression
  → Token Bucket Rate Limiter
  → Circuit Breaker
  → IP Filter
  → LRU Cache (HIT → return, MISS → continue)
  → Singleflight Dedup
  → Consistent Hash Ring → Origin
Response
```

---

## Milestones Completed

- [x] Milestone 1 — Reverse Proxy
- [x] Milestone 2 — Load Balancer + Health Checks
- [x] Milestone 3 — Edge Cache (LRU, TTL, stale-while-revalidate)
- [x] Milestone 4 — Security (rate limiting, circuit breaker, IP filtering)
- [x] Milestone 5 — Prometheus Metrics + Request ID Tracing
- [x] Milestone 6 — Live Admin Dashboard
- [x] Milestone 7 — Benchmarks + Performance Analysis
- [x] Milestone 8 — Gzip + Brotli Compression
- [x] Milestone 9 — Config Hot-Reload (zero downtime)
- [x] Milestone 10 — Consistent Hashing + Session Affinity
- [x] Milestone 11 — JWT Authentication
- [x] Milestone 12 — TLS Termination
- [x] Milestone 13 — OpenTelemetry Distributed Tracing
- [x] Milestone 14 — Grafana Dashboard
- [x] Milestone 15 — WebSocket Support
