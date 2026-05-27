<div align="center">

# EdgeFlow

**Production-style edge infrastructure platform built in Go**

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)
![Status](https://img.shields.io/badge/Status-Active-success?style=flat-square)
![Tests](https://img.shields.io/badge/Tests-Passing-brightgreen?style=flat-square)
![Features](https://img.shields.io/badge/Features-15-blue?style=flat-square)

*Reverse proxy · CDN caching · Load balancing · Security · Observability*

**95,290 req/sec** from cache &nbsp;·&nbsp; **<1ms p50 latency** &nbsp;·&nbsp; **87µs proxy overhead**

</div>

---

## Overview

EdgeFlow is a from-scratch edge proxy written in Go that combines the core responsibilities of a CDN, reverse proxy, and API gateway into a single cohesive system.

```
Client
  |
  v
+------------------------------------------+
|              EdgeFlow                    |
|                                          |
|  OpenTelemetry Tracing                   |
|  JWT Authentication                      |
|  Gzip / Brotli Compression               |
|  Rate Limiter · Circuit Breaker          |
|  LRU Cache + Stale-While-Revalidate      |
|  Singleflight Dedup                      |
|  Consistent Hash Ring                    |
|  Health Checks + Failover                |
+------------------------------------------+
  |           |           |
  v           v           v
Origin 1   Origin 2   Origin N
```

---

## Features

### Reverse Proxy
- HTTP/1.1 forwarding with connection pooling
- Header rewriting and response streaming
- Request ID tracing across all logs
- WebSocket support with HTTP upgrade and bidirectional streaming

### Load Balancer
- Round-robin and consistent hashing (150 virtual nodes per origin)
- Session affinity — same client always routes to same origin
- Active health checks with automatic failover and recovery
- Unhealthy origins removed from hash ring, re-added on recovery

### Edge Cache
- In-memory LRU with configurable capacity
- TTL expiry and stale-while-revalidate
- ETag / If-None-Match support
- Singleflight deduplication — prevents cache stampede under load
- Invalidation API by exact key or prefix
- `X-Cache: HIT / MISS / STALE` headers

### Security
- Token bucket rate limiter per IP
- Circuit breaker — opens after 5 failures, recovers after 30s
- IP allowlist / blocklist
- JWT authentication with HS256, role-based claims forwarded to origins
- Request body size limits and timeouts

### Compression
- Brotli (preferred) and Gzip (fallback)
- Automatic negotiation via `Accept-Encoding`
- Skips already-compressed content types

### TLS
- TLS 1.2+ with strong cipher suites (AES-256-GCM, AES-128-GCM)
- HTTP to HTTPS automatic redirect (301)
- Configurable certificate and key paths

### Config Hot-Reload
- Watches `config.json` every 2 seconds
- Zero-downtime updates to rate limits, blocklist, and body limits
- No restart required

### Observability
- Prometheus metrics at `/metrics`
- OpenTelemetry distributed tracing with span attributes and context propagation
- Structured JSON logs with request ID, method, path, status, and duration
- Live admin dashboard at `/dashboard` with 2-second refresh
- Grafana dashboard with Prometheus datasource

---

## Benchmark Results

> Tested with `wrk` · 4 threads · 50 connections · 10s duration · Apple Silicon

| Scenario | Req/sec | Avg Latency |
|---|---|---|
| Direct Origin | 121,824 | 498µs |
| EdgeFlow (cached) | 95,290 | 585µs |

- 87µs added overhead per proxied request
- 90%+ cache hit rate under normal traffic
- p50 latency under 1ms for cache hits
- p99 latency 5ms under load

---

## Quick Start

**Prerequisites:** Go 1.21+

```bash
# Clone and build
git clone https://github.com/anmol1505/edgeflow.git
cd edgeflow
go build ./...

# Start origin servers
go run benchmarks/origin1/main.go   # Terminal 1
go run benchmarks/origin2/main.go   # Terminal 2

# Start EdgeFlow
go run main.go
```

```bash
# Get a JWT token
curl -X POST http://localhost:8080/auth/token \
  -H "Content-Type: application/json" \
  -d '{"user_id": "alice", "role": "admin"}'

# Authenticated request (first = MISS, second = HIT)
TOKEN="your_token_here"
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/hello

# WebSocket
wscat -c ws://localhost:8080/ws

# Live dashboard
open http://localhost:8080/dashboard

# Prometheus metrics
curl http://localhost:8080/metrics | grep edgeflow
```

```bash
# Enable TLS
TLS_ENABLED=true go run main.go
curl -k https://localhost:8443/health
```

```bash
# Run tests
go test ./...
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

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Listen port |
| `TLS_ENABLED` | `false` | Enable HTTPS |
| `JWT_SECRET` | `edgeflow-secret-key` | JWT signing secret |
| `CERT_FILE` | `certs/cert.pem` | TLS certificate path |
| `KEY_FILE` | `certs/key.pem` | TLS private key path |

---

## API Reference

| Endpoint | Method | Auth | Description |
|---|---|---|---|
| `/health` | GET | No | Health, origin status, cache stats |
| `/metrics` | GET | No | Prometheus metrics |
| `/dashboard` | GET | No | Live admin dashboard |
| `/auth/token` | POST | No | Generate JWT token |
| `/admin/config` | GET | No | View current config |
| `/admin/cache/invalidate` | POST | No | Invalidate cache by key or prefix |
| `/ws` | WS | No | WebSocket proxy |
| `/*` | ANY | JWT | Proxied to origins |

---

## Project Structure

```
edgeflow/
├── main.go                     # Entry point and middleware pipeline
├── config.json                 # Hot-reloadable runtime config
├── certs/                      # TLS certificates
├── proxy/
│   ├── proxy.go                # HTTP forwarding and header rewriting
│   ├── compression.go          # Gzip and Brotli middleware
│   ├── tls.go                  # TLS termination
│   └── websocket.go            # WebSocket upgrade proxy
├── cache/
│   ├── cache.go                # LRU cache, TTL, eviction
│   ├── cache_test.go           # Cache unit tests
│   └── middleware.go           # Cache middleware and singleflight
├── lb/
│   ├── lb.go                   # Load balancer and health checks
│   ├── consistent_hash.go      # Consistent hash ring
│   └── consistent_hash_test.go # Hash ring unit tests
├── security/
│   ├── security.go             # Rate limiter, circuit breaker, IP filter
│   ├── ratelimiter_test.go     # Rate limiter unit tests
│   ├── circuitbreaker_test.go  # Circuit breaker unit tests
│   └── jwt.go                  # JWT middleware
├── observability/
│   ├── metrics.go              # Prometheus metric definitions
│   ├── middleware.go           # Request logging and metrics
│   ├── tracing.go              # OpenTelemetry setup
│   └── dashboard.go            # Live admin dashboard
├── control-plane/
│   └── config.go               # Config hot-reload watcher
└── benchmarks/
    ├── origin1/main.go         # Test origin server 1
    ├── origin2/main.go         # Test origin server 2
    ├── ws_server/main.go       # WebSocket test server
    ├── prometheus.yml          # Prometheus scrape config
    └── RESULTS.md              # Benchmark results
```

---

## Middleware Pipeline

```
Incoming Request
  |
  +-- OpenTelemetry Span
  +-- Prometheus Metrics + Structured Log
  +-- JWT Authentication
  +-- Gzip / Brotli Compression
  +-- Rate Limiter (token bucket per IP)
  +-- Circuit Breaker
  +-- IP Filter
  +-- LRU Cache lookup
  |     HIT --> return cached response
  |     MISS -> singleflight.Do()
  |               +-- Consistent Hash --> pick origin
  |                     +-- HTTP / WebSocket forward
  +-- Response + cache store
```

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.21+ |
| Metrics | Prometheus `client_golang` |
| Tracing | OpenTelemetry SDK |
| Auth | `golang-jwt/jwt` |
| Compression | `andybalholm/brotli` + stdlib gzip |
| Deduplication | `golang.org/x/sync/singleflight` |
| Dashboards | Grafana + Prometheus |
| Load Testing | `wrk` |

---

## Milestones

- [x] Reverse proxy — HTTP forwarding, headers, structured logging
- [x] Load balancer — round-robin, health checks, automatic failover
- [x] Edge cache — LRU, TTL, stale-while-revalidate
- [x] Security — rate limiting, circuit breaker, IP filtering
- [x] Observability — Prometheus metrics, request ID tracing
- [x] Admin dashboard — live stats with 2-second refresh
- [x] Benchmarks — wrk load tests, results documented
- [x] Compression — gzip and brotli with encoding negotiation
- [x] Config hot-reload — zero downtime config updates
- [x] Consistent hashing — virtual nodes, session affinity
- [x] JWT authentication — HS256, role claims, public path exclusions
- [x] TLS termination — TLS 1.2+, HTTP to HTTPS redirect
- [x] OpenTelemetry tracing — spans, attributes, context propagation
- [x] Grafana dashboard — Prometheus datasource, live graphs
- [x] WebSocket support — HTTP upgrade, bidirectional streaming
- [x] Unit tests — cache, rate limiter, circuit breaker, consistent hashing

---

## License

MIT License — see [LICENSE](LICENSE) for details.
