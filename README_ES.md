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
  <strong>Proxy inverso HTTP offline-first con caché determinística LRU + WAL.</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/Cache-LRU_%2B_WAL-teal?style=for-the-badge" alt="Cache">
  <img src="https://img.shields.io/badge/Concurrency-Race_Tested-success?style=for-the-badge" alt="Race Tested">
</p>

Mockbird se coloca entre tu aplicación y una API real. Registra respuestas del upstream y las reproduce desde una caché local cuando estás sin conexión, limitado por rate limits o ejecutando pruebas repetibles.

## Inicio rápido

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

Luego apunta tu aplicación a:

```text
http://127.0.0.1:8080
```

## Características

| Área | Detalles |
| --- | --- |
| Proxy HTTP | Proxy inverso, preserva códigos de estado y cabeceras |
| Caché L1 | LRU en RAM con TTL y acceso sincronizado |
| Persistencia L2 | `cache.wal` append-only con replay en arranque |
| Claves | SHA-256 determinístico con método, path normalizado, query ordenada, body hash y `-vary` |
| Seguridad operativa | Límites de cuerpo, timeouts, `log/slog`, apagado graceful |

## API de gestión

```bash
curl http://127.0.0.1:8080/__mockbird/cache
curl -X DELETE http://127.0.0.1:8080/__mockbird/cache
curl -X DELETE http://127.0.0.1:8080/__mockbird/cache/{key}
```

## Pruebas

```bash
go test ./...
go test -race ./...
go build ./cmd/mockbird
```

Lee [README.md](README.md) para la documentación completa.
