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
  <strong>Reverse proxy HTTP offline-first con cache deterministica LRU + WAL.</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/Cache-LRU_%2B_WAL-teal?style=for-the-badge" alt="Cache">
  <img src="https://img.shields.io/badge/Concurrency-Race_Tested-success?style=for-the-badge" alt="Race Tested">
</p>

Mockbird si posiziona tra la tua applicazione e una vera API upstream. Registra le risposte e le riproduce da una cache locale quando sei offline, sotto rate limit o durante test ripetibili.

## Avvio rapido

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

Poi usa:

```text
http://127.0.0.1:8080
```

## Funzionalità

| Area | Dettagli |
| --- | --- |
| Proxy HTTP | Reverse proxy, preserva status code e header |
| Cache L1 | LRU in RAM con TTL e accesso sincronizzato |
| Persistenza L2 | `cache.wal` append-only con replay all'avvio |
| Chiavi | SHA-256 deterministico con metodo, path normalizzato, query ordinata, hash del body e `-vary` |
| Produzione | Limiti sui body, timeout, `log/slog`, shutdown graceful |

## API di gestione

```bash
curl http://127.0.0.1:8080/__mockbird/cache
curl -X DELETE http://127.0.0.1:8080/__mockbird/cache
curl -X DELETE http://127.0.0.1:8080/__mockbird/cache/{key}
```

## Test

```bash
go test ./...
go test -race ./...
go build ./cmd/mockbird
```

Leggi [README.md](README.md) per la documentazione completa.
