# Mockbird

Mockbird is a lightweight HTTP reverse proxy that records upstream responses and replays them from a deterministic two-level cache:

- L1 RAM cache: bounded LRU for hot responses.
- L2 disk cache: append-only `cache.wal` records with full response metadata.

Correctness rules: cache keys are deterministic, stored entries are immutable, and runtime formatting flags never change persisted cache bytes.

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
  -vary=Authorization,Accept \
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
| `-max-body` | `10485760` | Maximum request body bytes read for hashing and proxying. |
| `-max-response` | `104857600` | Maximum upstream response bytes to record. |
| `-vary` | empty | Comma-separated request headers included in cache keys. |
| `-pretty` | `false` | Pretty-print JSON only at response time. Stored cache bytes stay raw. |
| `-log-level` | `info` | `debug`, `info`, `warn`, or `error`. |

## Cache Keys

Cache keys are filesystem-safe SHA-256 hex strings built from:

- HTTP method
- normalized path
- sorted query parameters and sorted repeated query values
- SHA-256 body hash for `POST`, `PUT`, and `PATCH`
- configured request headers from `-vary`

Examples:

- `GET /users?a=1&b=2` and `GET /users?b=2&a=1` produce the same key.
- `GET /users?id=1` and `GET /users?id=2` produce different keys.
- `POST /login` with different bodies produces different keys.
- `-vary=Authorization` separates otherwise identical requests by `Authorization`.

## Cache Format

Disk persistence is a single append-only WAL file named `cache.wal`. Each line is one JSON record:

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

`body` is base64 because Go encodes `[]byte` that way in JSON. This preserves binary, compressed, and non-JSON responses.

Startup replays `cache.wal` sequentially into an in-memory index. Corrupted partial tail records are skipped, so an interrupted append does not poison recovery. A background compactor periodically rewrites the WAL into the current live `set` records, removing stale duplicates and keeping the file bounded.

## Cache Management API

Mockbird reserves `__mockbird` paths for cache operations:

```bash
curl http://127.0.0.1:8080/__mockbird/cache
curl -X DELETE http://127.0.0.1:8080/__mockbird/cache
curl -X DELETE http://127.0.0.1:8080/__mockbird/cache/{key}
```

## Architecture Notes

- `cmd/mockbird` only parses config, creates dependencies, wires routes, and handles shutdown.
- `internal/proxy` owns HTTP behavior: request body limits, upstream URL construction, header forwarding, response replay, and singleflight request deduplication.
- `internal/cache` owns pure cache semantics: keys, TTL, LRU, RAM/disk coordination, invalidation, and snapshots.
- `internal/storage` owns persistence: append-only WAL writes, startup replay, corrupted-tail tolerance, and background compaction.
- `internal/server` owns management routes only.

LRU eviction removes the corresponding disk entry as well. In this design, `-max-ram` bounds the active cache set rather than allowing disk to grow without limit behind RAM.

## Migration Notes

- `main.go` moved to `cmd/mockbird/main.go`; run with `go run ./cmd/mockbird`.
- Old cache files named like `GET_users.json` are not compatible. The new cache uses deterministic SHA-256 filenames.
- `-pretty` no longer changes stored cache files. It only formats JSON while writing the HTTP response and never touches non-JSON or encoded payloads.
- Query parameter order no longer creates duplicate entries.
- Cache persistence now uses `cache.wal`; old per-entry `.json` cache files and `index.json` are ignored by the new storage layer.

## Test

```bash
go test ./...
go test -race ./...
```
