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
  <strong>Reverse proxy HTTP offline-first com cache determinístico LRU + WAL.</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/Cache-LRU_%2B_WAL-teal?style=for-the-badge" alt="Cache">
  <img src="https://img.shields.io/badge/Concurrency-Race_Tested-success?style=for-the-badge" alt="Race Tested">
</p>

Mockbird fica entre sua aplicação e uma API upstream real. Ele grava respostas e as reproduz de um cache local quando você está offline, limitado por rate limit ou rodando testes reproduzíveis.

## Início rápido

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

Depois use:

```text
http://127.0.0.1:8080
```

## Recursos

| Área | Detalhes |
| --- | --- |
| Proxy HTTP | Reverse proxy, preserva status codes e headers |
| Cache L1 | LRU em RAM com TTL e acesso sincronizado |
| Persistência L2 | `cache.wal` append-only com replay na inicialização |
| Chaves | SHA-256 determinístico com método, path normalizado, query ordenada, hash do body e `-vary` |
| Produção | Limites de body, timeouts, `log/slog`, shutdown graceful |

## API de gerenciamento

```bash
curl http://127.0.0.1:8080/__mockbird/cache
curl -X DELETE http://127.0.0.1:8080/__mockbird/cache
curl -X DELETE http://127.0.0.1:8080/__mockbird/cache/{key}
```

## Testes

```bash
go test ./...
go test -race ./...
go build ./cmd/mockbird
```

Leia [README.md](README.md) para a documentação completa.
