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
  <strong>Proxy inverse HTTP offline-first avec cache déterministe LRU + WAL.</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/Cache-LRU_%2B_WAL-teal?style=for-the-badge" alt="Cache">
  <img src="https://img.shields.io/badge/Concurrency-Race_Tested-success?style=for-the-badge" alt="Race Tested">
</p>

Mockbird se place entre votre application et une API réelle. Il enregistre les réponses upstream et les rejoue depuis un cache local lorsque vous êtes hors ligne, limité par des quotas ou en train d'exécuter des tests reproductibles.

## Démarrage rapide

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

Utilisez ensuite:

```text
http://127.0.0.1:8080
```

## Fonctionnalités

| Domaine | Détails |
| --- | --- |
| Proxy HTTP | Proxy inverse, préserve les statuts et les en-têtes |
| Cache L1 | LRU en mémoire avec TTL et accès synchronisé |
| Persistance L2 | `cache.wal` append-only avec replay au démarrage |
| Clés | SHA-256 déterministe avec méthode, chemin normalisé, query triée, hash du corps et `-vary` |
| Production | Limites de corps, timeouts, `log/slog`, arrêt gracieux |

## API de gestion

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

Consultez [README.md](README.md) pour la documentation complète.
