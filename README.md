# Mockbird

Mockbird is a reverse-proxy offline API recorder for local development and tests. It forwards requests to an upstream API, records successful responses, and can replay them later from a two-level cache:

- L1 RAM cache: bounded LRU for hot responses.
- L2 disk cache: persistent JSON files containing full response metadata.

## Project Structure

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

## Run

```bash
go run ./cmd/mockbird \
  -target=https://api.example.com \
  -port=8080 \
  -dir=./mockbird_cache \
  -ttl=24h \
  -max-ram=1000 \
  -log-level=info
```

## CLI Flags

| Flag | Default | Description |
| --- | --- | --- |
| `-target` | `https://jsonplaceholder.typicode.com` | Upstream API origin, optionally with a path prefix. |
| `-port` | `8080` | Local listen port on `127.0.0.1`. |
| `-dir` | `./mockbird_cache` | L2 disk cache directory. |
| `-ttl` | `24h` | Cache entry lifetime. Expired entries are ignored and refreshed. |
| `-max-ram` | `1000` | Maximum number of L1 RAM cache entries. |
| `-pretty` | `false` | Pretty-print JSON payloads only. Non-JSON payloads are never changed. |
| `-log-level` | `info` | `debug`, `info`, `warn`, or `error`. |

## Cache Format

Each disk entry is stored as `<sha256>.json` and contains status, headers, creation time, and body bytes:

```json
{
  "status_code": 200,
  "headers": {
    "Content-Type": ["application/json"],
    "ETag": ["\"abc\""]
  },
  "body": "eyJvayI6dHJ1ZX0=",
  "created_at": "2026-05-17T12:00:00Z"
}
```

`body` is base64 because it is a Go `[]byte` encoded as JSON. This preserves binary and non-JSON responses.

## Cache Keys

Cache keys are filesystem-safe SHA-256 hex strings built from:

- HTTP method
- request path
- raw query string
- SHA-256 body hash for `POST`, `PUT`, and `PATCH`

That means `GET /users?id=1`, `GET /users?id=2`, and two `POST /login` calls with different bodies produce different cache entries.

## Cache Management API

Mockbird reserves `__mockbird` paths for cache operations:

```bash
curl http://127.0.0.1:8080/__mockbird/cache
curl -X DELETE http://127.0.0.1:8080/__mockbird/cache
curl -X DELETE http://127.0.0.1:8080/__mockbird/cache/{key}
```

## Architectural Decisions

- The proxy stores a `CacheEntry`, not just a body, so cached responses replay original status codes and headers such as `Location`, `ETag`, `Cache-Control`, and `Content-Type`.
- RAM cache uses `github.com/hashicorp/golang-lru/v2`, preventing unbounded memory growth.
- Disk cache writes full metadata atomically through a temp file and rename.
- `golang.org/x/sync/singleflight` deduplicates concurrent cache misses for the same key.
- `http.Server` plus signal handling provides graceful shutdown on Ctrl+C and SIGTERM.
- `log/slog` replaces terminal animation, emoji logging, and decorative output with structured levels.
- The HTTP transport has dial, TLS handshake, idle connection, response header, and client timeouts.

## Migration Notes

- `main.go` moved to `cmd/mockbird/main.go`; run with `go run ./cmd/mockbird`.
- Old cache files named like `GET_users.json` are not compatible. The new format uses hashed keys and stores full HTTP metadata.
- Automatic JSON formatting is disabled by default. Use `-pretty` when readable JSON cache files are more important than byte-for-byte body preservation.
- Cache clearing no longer requires deleting the directory manually; use `DELETE /__mockbird/cache`.
- Logs are structured text records controlled by `-log-level`.

## Test

```bash
go test ./...
```
