<div align="center">

# ⚡ EdgeFlow

**A production-style edge infrastructure platform built in Go**

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)
![Status](https://img.shields.io/badge/Status-Active-success?style=flat-square)
![Features](https://img.shields.io/badge/Features-15-blue?style=flat-square)

*Reverse proxy · CDN caching · Load balancing · Security · Observability*

**95,290 req/sec** from cache &nbsp;·&nbsp; **<1ms p50 latency** &nbsp;·&nbsp; **87µs proxy overhead**

</div>

---

## Overview

EdgeFlow is a from-scratch edge proxy written in Go that combines the core responsibilities of a CDN, reverse proxy, and API gateway into a single cohesive system. Built to demonstrate real infrastructure engineering — not just a tutorial project.

```
Client
  │
  ▼
┌─────────────────────────────────────────┐
│              EdgeFlow                   │
│                                         │
│  OpenTelemetry Tracing                  │
│  JWT Authentication                     │
│  Gzip / Brotli Compression              │
│  Rate Limiter · Circuit Breaker         │
│  LRU Cache + Stale-While-Revalidate     │
│  Singleflight Dedup                     │
│  Consistent Hash Ring                   │
│  Health Checks + Failover               │
└─────────────────────────────────────────┘
  │           │           │
  ▼           ▼           ▼
Origin 1   Origin 2   Origin N
```

---

## Features

### 🔀 Reverse Proxy
- HTTP/1.1 forwarding with connection pooling
- Header rewriting and response streaming
- Request ID tracing across all logs
- WebSocket support with HTTP upgrade and bidirectional streaming

### ⚖️ Load Balancer
- Round-robin and **consistent hashing** (150 virtual nodes per origin)
- Session affinity — same client always routes to same origin
- Active health checks with automatic failover and recovery
- Unhealthy origins removed from hash ring, re-added on recovery

### 🗄️ Edge Cache
- In-memory LRU with configurable capacity
- TTL expiry + **stale-while-revalidate**
- ETag / If-None-Match support
- **Singleflight deduplication** — prevents cache stampede under load
- Invalidation API by exact key or prefix
- `X-Cache: HIT / MISS / STALE` headers

### 🔒 Security
- **Token bucket rate limiter** per IP
- **Circuit breaker** — opens after 5 failures, recovers after 30s
- IP allowlist / blocklist
- **JWT authentication** with HS256, role-based claims
- Request body size limits and timeouts

### 🗜️ Compression
- Brotli (preferred) + Gzip (fallback)
- Automatic negotiation via `Accept-Encoding`
- Skips already-compressed content types

### 🔐 TLS
- TLS 1.2+ with strong cipher suites
- HTTP → HTTPS automatic redirect
- Configurable cert/key paths

### 🔄 Config Hot-Reload
- Watches `config.json` every 2 seconds
- Zero-downtime updates to rate limits, blocklist, body limits
- No restart required

### 📊 Observability
- Prometheus metrics at `/metrics`
- OpenTelemetry distributed tracing with span attributes
- Structured JSON logs with request ID and cache status
- Live admin dashboard at `/dashboard` (2s refresh)
- Grafana dashboard with Prometheus datasource

---

## Benchmark Results

> Tested with `wrk` · 4 threads · 50 connections · 10s duration · Apple Silicon

| Scenario | Req/sec | Avg Latency |
|---|---|---|
| 🟢 Direct Origin | 121,824 | 498µs |
| ⚡ EdgeFlow Cached | 95,290 | 585µs |

- **87µs** added overhead per proxied request
- **90%+** cache hit rate under normal traffic
- **<1ms** p50 latency for cache hits
- p99 latency: **5ms** under load

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

# Make authenticated requests (1st = MISS, 2nd = HIT)
TOKEN="your_token_here"
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/hello

# WebSocket
wscat -c ws://localhost:8080/ws

# Live dashboard
open http://localhost:8080/dashboard
```

```bash
# Enable TLS
TLS_ENABLED=true go run main.go
curl -k https://localhost:8443/health
```

---

## Configuration

`config.json` is hot-reloaded every 2 seconds — no restart needed:

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
| `CERT_FILE` | `certs/cert.pem` | TLS certificate |
| `KEY_FILE` | `certs/key.pem` | TLS private key |

---

## API Reference

| Endpoint | Method | Auth | Description |
|---|---|---|---|
| `/health` | GET | ✗ | Health, origins, cache stats |
| `/metrics` | GET | ✗ | Prometheus metrics |
| `/dashboard` | GET | ✗ | Live admin dashboard |
| `/auth/token` | POST | ✗ | Generate JWT token |
| `/admin/config` | GET | ✗ | Current config |
| `/admin/cache/invalidate` | POST | ✗ | Invalidate cache |
| `/ws` | WS | ✗ | WebSocket proxy |
| `/*` | ANY | ✓ | Proxied to origins |

---

## Project Structure

```
edgeflow/
├── main.go                     # Entry point + middleware pipeline
├── config.json                 # Hot-reloadable runtime config
├── certs/                      # TLS certificates
├── proxy/
│   ├── proxy.go                # HTTP forwarding
│   ├── compression.go          # Gzip/Brotli middleware
│   ├── tls.go                  # TLS termination
│   └── websocket.go            # WebSocket upgrade proxy
├── cache/
│   ├── cache.go                # LRU + TTL + eviction
│   └── middleware.go           # Cache middleware + singleflight
├── lb/
│   ├── lb.go                   # Load balancer + health checks
│   └── consistent_hash.go      # Consistent hash ring
├── security/
│   ├── security.go             # Rate limiter + circuit breaker
│   └── jwt.go                  # JWT middleware
├── observability/
│   ├── metrics.go              # Prometheus metrics
│   ├── middleware.go           # Logging + metrics
│   ├── tracing.go              # OpenTelemetry
│   └── dashboard.go            # Live dashboard
├── control-plane/
│   └── config.go               # Config watcher
└── benchmarks/
    ├── origin1/main.go
    ├── origin2/main.go
    ├── ws_server/main.go
    ├── prometheus.yml
    └── RESULTS.md
```

---

## Middleware Pipeline

```
Incoming Request
  │
  ├─► OpenTelemetry Span (trace ID, attributes)
  ├─► Prometheus Metrics + Structured Log
  ├─► JWT Auth (skip public paths)
  ├─► Gzip/Brotli Compression
  ├─► Rate Limiter (token bucket per IP)
  ├─► Circuit Breaker
  ├─► IP Filter
  ├─► LRU Cache lookup
  │     HIT ──► return cached response
  │     MISS ─► singleflight.Do()
  │               └─► Consistent Hash → pick origin
  │                     └─► HTTP/WS forward
  └─► Response + cache store
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
| Dedup | `golang.org/x/sync/singleflight` |
| Dashboards | Grafana + Prometheus |
| Load Testing | `wrk` |

---

## Milestones

- [x] Reverse Proxy — HTTP forwarding, headers, logging
- [x] Load Balancer — round-robin, health checks, failover
- [x] Edge Cache — LRU, TTL, stale-while-revalidate
- [x] Security — rate limiting, circuit breaker, IP filtering
- [x] Observability — Prometheus metrics, request tracing
- [x] Admin Dashboard — live stats, 2s refresh
- [x] Benchmarks — wrk load tests, results documented
- [x] Compression — gzip + brotli with encoding negotiation
- [x] Config Hot-Reload — zero downtime config updates
- [x] Consistent Hashing — virtual nodes, session affinity
- [x] JWT Authentication — HS256, role claims, public paths
- [x] TLS Termination — TLS 1.2+, HTTP redirect
- [x] OpenTelemetry Tracing — spans, attributes, propagation
- [x] Grafana Dashboard — Prometheus datasource, live graphs
- [x] WebSocket Support — HTTP upgrade, bidirectional proxy

---

<div align="center">

Built with Go · No frameworks · No shortcuts

</div>
