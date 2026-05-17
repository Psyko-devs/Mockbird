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
  <strong>Offline-first HTTP reverse proxy recorder с детерминированным кэшем LRU + WAL.</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/Cache-LRU_%2B_WAL-teal?style=for-the-badge" alt="Cache">
  <img src="https://img.shields.io/badge/Concurrency-Race_Tested-success?style=for-the-badge" alt="Race Tested">
</p>

Mockbird работает между вашим приложением и реальным upstream API. Он записывает ответы и воспроизводит их из локального кэша при offline-разработке, rate limit или повторяемых тестах.

## Быстрый старт

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

Затем используйте:

```text
http://127.0.0.1:8080
```

## Возможности

| Область | Детали |
| --- | --- |
| HTTP Proxy | Reverse proxy, сохраняет status codes и headers |
| L1 Cache | LRU в RAM с TTL и синхронизированным доступом |
| L2 Persistence | Append-only `cache.wal`, replay при запуске |
| Cache Keys | Детерминированный SHA-256 из method, normalized path, sorted query, body hash и `-vary` |
| Production | Body limits, timeouts, `log/slog`, graceful shutdown |

## Management API

```bash
curl http://127.0.0.1:8080/__mockbird/cache
curl -X DELETE http://127.0.0.1:8080/__mockbird/cache
curl -X DELETE http://127.0.0.1:8080/__mockbird/cache/{key}
```

## Тесты

```bash
go test ./...
go test -race ./...
go build ./cmd/mockbird
```

Полная документация доступна в [README.md](README.md).
