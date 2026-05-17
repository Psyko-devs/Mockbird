<p align="center">
  <a href="README.md">English</a> |
  <a href="README_CN.md">简体中文</a> |
  <a href="README_TW.md">繁體中文</a> |
  <a href="README_JA.md">日本語</a> |
  <a href="README_KO.md">한국어</a> |
  <a href="README_FR.md">Français</a> |
  <a href="README_ES.md">Español</a> |
  <a href="README_DE.md">Deutsch</a> |
  <a href="README_IT.md">Italiano</a> |
  <a href="README_RU.md">Русский</a> |
  <a href="README_PT-BR.md">Português (Brasil)</a>
</p>

<h1 align="center">Mockbird</h1>

<p align="center">
  <strong>Offline-first HTTP reverse proxy recorder with deterministic L1/L2 caching.</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/Proxy-Reverse_Proxy-blue?style=for-the-badge" alt="Reverse Proxy">
  <img src="https://img.shields.io/badge/Cache-LRU_%2B_WAL-teal?style=for-the-badge" alt="LRU plus WAL">
  <img src="https://img.shields.io/badge/Concurrency-Race_Tested-success?style=for-the-badge" alt="Race Tested">
</p>

<p align="center">
  <a href="#quick-start"><img src="https://img.shields.io/badge/Quick_Start-60_seconds-blue" alt="Quick Start"></a>
  <a href="#cache-correctness"><img src="https://img.shields.io/badge/Keys-Deterministic-purple" alt="Deterministic Cache Keys"></a>
  <a href="#wal-storage"><img src="https://img.shields.io/badge/Storage-Append_Only_WAL-orange" alt="WAL Storage"></a>
  <a href="#testing"><img src="https://img.shields.io/badge/Tests-go_test_%2B_race-brightgreen" alt="Tests"></a>
</p>

---

Mockbird sits between your application and a real upstream API. It records successful upstream responses, stores the complete HTTP response metadata, and replays cached responses when you are offline, rate-limited, or running repeatable tests.

It is designed as a lightweight local proxy, inspired by the practical parts of HTTP caches like Varnish and Squid, but small enough to run with one Go command.

## Why Mockbird?

| Problem | Mockbird answer |
| --- | --- |
| Third-party API is offline | Replay cached responses locally |
| API rate limits slow development | Hit Mockbird instead of the upstream |
| Tests need repeatable HTTP responses | Deterministic keys and immutable cache entries |
| Large projects need safe concurrency | LRU access is synchronized and race-tested |
| Disk cache must survive crashes | Append-only WAL recovery skips corrupted tails |

---

## Features

<table>
<tr>
<td width="50%">

### HTTP Proxy
- Reverse proxy to any HTTP API
- Preserves status codes and response headers
- Replays cached responses with correct HTTP metadata
- Optional JSON pretty output at response time only
- Request coalescing with `singleflight`

</td>
<td width="50%">

### Cache Engine
- L1 RAM cache with bounded LRU
- L2 append-only `cache.wal`
- TTL-aware expiration
- SHA-256 deterministic cache keys
- Vary-style request header support

</td>
</tr>
<tr>
<td width="50%">

### Production Safety
- Graceful shutdown on Ctrl+C and SIGTERM
- Configurable request and response body limits
- Structured `log/slog` logging
- HTTP transport timeouts
- Race-tested concurrency path

</td>
<td width="50%">

### Operations
- Cache inspection API
- Clear all cache
- Delete one cache entry
- WAL replay on startup
- Background WAL compaction

</td>
</tr>
</table>

---

## Quick Start

```bash
go run ./cmd/mockbird \
  -target=https://api.example.com \
  -port=8080 \
  -dir=./mockbird_cache \
  -ttl=24h \
  -max-ram=1000 \
  -vary=Authorization,Accept \
  -log-level=info
```

Then point your app at:

```text
http://127.0.0.1:8080
```

Example:

```bash
curl http://127.0.0.1:8080/users?id=1
```

First request fetches from the upstream and records the response. Later requests replay from RAM or the recovered WAL-backed cache.

---

## CLI Reference

| Flag | Default | Description |
| --- | --- | --- |
| `-target` | `https://jsonplaceholder.typicode.com` | Upstream API origin, optionally with a path prefix. |
| `-port` | `8080` | Local listen port on `127.0.0.1`. |
| `-dir` | `./mockbird_cache` | Cache directory containing `cache.wal`. |
| `-ttl` | `24h` | Cache entry lifetime. Expired entries are ignored and refreshed. |
| `-max-ram` | `1000` | Maximum active cache entries. LRU eviction also removes the L2 record. |
| `-max-body` | `10485760` | Maximum request body bytes read for hashing and proxying. |
| `-max-response` | `104857600` | Maximum upstream response bytes to record. |
| `-vary` | empty | Comma-separated request headers included in cache keys. |
| `-pretty` | `false` | Pretty-print JSON only while writing the HTTP response. Stored bytes stay raw. |
| `-log-level` | `info` | `debug`, `info`, `warn`, or `error`. |

---

## Cache Correctness

Cache keys are filesystem-safe SHA-256 hex strings built from:

- HTTP method
- normalized path
- sorted query parameters
- sorted repeated query values
- SHA-256 body hash for `POST`, `PUT`, and `PATCH`
- configured request headers from `-vary`

Examples:

| Request | Result |
| --- | --- |
| `GET /users?a=1&b=2` |
| `GET /users?b=2&a=1` | Same cache key |
| `GET /users?id=1` |
| `GET /users?id=2` | Different cache keys |
| `POST /login` with body `A` |
| `POST /login` with body `B` | Different cache keys |
| `-vary=Authorization` | Separates entries by `Authorization` value |

Runtime flags do not mutate stored entries. For example, `-pretty` only changes the bytes written to the client response; `cache.wal` keeps the original upstream body.

---

## WAL Storage

Disk persistence is a single append-only file:

```text
mockbird_cache/
└── cache.wal
```

Each WAL line is a JSON record:

```json
{
  "op": "set",
  "key": "64-character-sha256-key",
  "entry": {
    "status_code": 200,
    "headers": {
      "Content-Type": ["application/json"],
      "ETag": ["\"abc\""]
    },
    "body": "eyJvayI6dHJ1ZX0=",
    "created_at": "2026-05-17T12:00:00Z"
  },
  "timestamp": 1779028800000000000
}
```

Supported operations:

| Operation | Meaning |
| --- | --- |
| `set` | Add or replace a cache entry |
| `delete` | Remove one cache entry |
| `clear` | Remove all cache entries |

On startup, Mockbird replays `cache.wal` sequentially into memory. If the process crashes during an append, a corrupted partial tail record is skipped. A background compactor periodically rewrites the WAL into the current live `set` records so the file stays bounded.

---

## Cache Management API

Mockbird reserves the `__mockbird` path namespace:

```bash
# Inspect entries
curl http://127.0.0.1:8080/__mockbird/cache

# Clear all entries
curl -X DELETE http://127.0.0.1:8080/__mockbird/cache

# Delete one entry
curl -X DELETE http://127.0.0.1:8080/__mockbird/cache/{key}
```

---

## Architecture

```text
Client
  |
  v
Mockbird HTTP server
  |
  +-- internal/server   management routes
  +-- internal/proxy    reverse proxy, replay, request coalescing
  +-- internal/cache    deterministic keys, TTL, synchronized LRU
  +-- internal/storage  append-only WAL, replay, compaction
  |
  v
Upstream API
```

Project layout:

```text
mockbird/
├── cmd/mockbird/main.go
├── internal/cache/
│   ├── entry.go
│   ├── key.go
│   └── manager.go
├── internal/config/config.go
├── internal/proxy/proxy.go
├── internal/server/cache_api.go
└── internal/storage/disk.go
```

---

## Testing

```bash
go test ./...
go test -race ./...
go build ./cmd/mockbird
```

Covered behavior includes:

- deterministic key generation
- Vary header semantics
- body hashing for mutation methods
- TTL expiration
- LRU eviction and L2 consistency
- WAL replay
- delete and clear replay
- corrupted WAL tail recovery
- concurrent cache/store access
- offline replay
- preserved status codes and headers

---

## Migration Notes

- `main.go` moved to `cmd/mockbird/main.go`.
- Old cache files named like `GET_users.json` are not compatible.
- The old per-entry `.json` and `index.json` persistence model is replaced by `cache.wal`.
- Query parameter order no longer creates duplicate entries.
- `-pretty` no longer changes stored cache files.
- `-max-ram` bounds the active cache set; LRU eviction also appends a delete to the WAL.

---

## Roadmap

- Metrics endpoint for hit rate, miss rate, WAL size, and upstream latency
- Optional stale-while-revalidate mode
- Optional auth guard for management endpoints
- Streaming passthrough for explicitly non-cacheable large responses
- Config file support in addition to CLI flags

---

<div align="center">

**Mockbird** — deterministic offline HTTP replay for developers.

</div>
