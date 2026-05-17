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
  <strong>Offline-first HTTP-Reverse-Proxy mit deterministischem LRU + WAL Cache.</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/Cache-LRU_%2B_WAL-teal?style=for-the-badge" alt="Cache">
  <img src="https://img.shields.io/badge/Concurrency-Race_Tested-success?style=for-the-badge" alt="Race Tested">
</p>

Mockbird steht zwischen Ihrer Anwendung und einer echten Upstream-API. Es zeichnet Antworten auf und spielt sie später aus einem lokalen Cache wieder ab, etwa offline, bei Rate Limits oder in reproduzierbaren Tests.

## Schnellstart

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

Danach verwenden Sie:

```text
http://127.0.0.1:8080
```

## Funktionen

| Bereich | Details |
| --- | --- |
| HTTP Proxy | Reverse Proxy, bewahrt Statuscodes und Header |
| L1 Cache | LRU im RAM mit TTL und synchronisiertem Zugriff |
| L2 Persistenz | Append-only `cache.wal` mit Replay beim Start |
| Schlüssel | Deterministisches SHA-256 aus Methode, normalisiertem Pfad, sortierter Query, Body-Hash und `-vary` |
| Betrieb | Body-Limits, Timeouts, `log/slog`, Graceful Shutdown |

## Management API

```bash
curl http://127.0.0.1:8080/__mockbird/cache
curl -X DELETE http://127.0.0.1:8080/__mockbird/cache
curl -X DELETE http://127.0.0.1:8080/__mockbird/cache/{key}
```

## Tests

```bash
go test ./...
go test -race ./...
go build ./cmd/mockbird
```

Die vollständige Dokumentation finden Sie in [README.md](README.md).
