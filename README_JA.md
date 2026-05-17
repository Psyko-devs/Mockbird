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
  <strong>決定論的な LRU + WAL キャッシュを備えた offline-first HTTP リバースプロキシレコーダー。</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/Cache-LRU_%2B_WAL-teal?style=for-the-badge" alt="Cache">
  <img src="https://img.shields.io/badge/Concurrency-Race_Tested-success?style=for-the-badge" alt="Race Tested">
</p>

Mockbird はアプリケーションと実際の upstream API の間に入り、レスポンスを記録します。オフライン時、rate limit 時、再現性のあるテスト時にローカルキャッシュからレスポンスを再生できます。

## クイックスタート

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

アプリケーションは次を使用します。

```text
http://127.0.0.1:8080
```

## 機能

| 領域 | 詳細 |
| --- | --- |
| HTTP Proxy | リバースプロキシ、ステータスコードとヘッダーを保持 |
| L1 Cache | TTL 付きのメモリ LRU、同期済みアクセス |
| L2 Persistence | append-only `cache.wal`、起動時 replay |
| Cache Keys | method、正規化 path、ソート済み query、body hash、`-vary` による決定論的 SHA-256 |
| Production | body 制限、timeout、`log/slog`、graceful shutdown |

## 管理 API

```bash
curl http://127.0.0.1:8080/__mockbird/cache
curl -X DELETE http://127.0.0.1:8080/__mockbird/cache
curl -X DELETE http://127.0.0.1:8080/__mockbird/cache/{key}
```

## テスト

```bash
go test ./...
go test -race ./...
go build ./cmd/mockbird
```

完全なドキュメントは [README.md](README.md) を参照してください。
