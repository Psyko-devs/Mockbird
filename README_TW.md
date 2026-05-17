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
  <strong>離線優先的 HTTP 反向代理記錄器，採用確定性的 LRU + WAL 快取。</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/Cache-LRU_%2B_WAL-teal?style=for-the-badge" alt="Cache">
  <img src="https://img.shields.io/badge/Concurrency-Race_Tested-success?style=for-the-badge" alt="Race Tested">
</p>

Mockbird 位於你的應用程式與真實上游 API 之間。它會記錄上游回應，並在離線、遭遇 rate limit 或執行可重現測試時從本機快取重放。

## 快速開始

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

然後讓應用程式使用：

```text
http://127.0.0.1:8080
```

## 功能

| 模組 | 說明 |
| --- | --- |
| HTTP Proxy | 反向代理，保留狀態碼與回應標頭 |
| L1 快取 | 具 TTL 的記憶體 LRU，並發安全 |
| L2 持久化 | 追加式 `cache.wal`，啟動時 replay |
| 快取鍵 | 方法、正規化路徑、排序 query、body hash 與 `-vary` 組成的確定性 SHA-256 |
| 生產能力 | body 限制、timeout、`log/slog`、優雅關閉 |

## 管理 API

```bash
curl http://127.0.0.1:8080/__mockbird/cache
curl -X DELETE http://127.0.0.1:8080/__mockbird/cache
curl -X DELETE http://127.0.0.1:8080/__mockbird/cache/{key}
```

## 測試

```bash
go test ./...
go test -race ./...
go build ./cmd/mockbird
```

完整文件請閱讀 [README.md](README.md)。
