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
  <strong>离线优先的 HTTP 反向代理记录器，使用确定性的 LRU + WAL 缓存。</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/Cache-LRU_%2B_WAL-teal?style=for-the-badge" alt="Cache">
  <img src="https://img.shields.io/badge/Concurrency-Race_Tested-success?style=for-the-badge" alt="Race Tested">
</p>

Mockbird 位于你的应用和真实上游 API 之间。它会记录上游响应，并在离线、被限流或运行可重复测试时从本地缓存重放这些响应。

## 快速开始

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

然后让应用访问：

```text
http://127.0.0.1:8080
```

## 功能

| 模块 | 说明 |
| --- | --- |
| HTTP Proxy | 反向代理，保留状态码和响应头 |
| L1 缓存 | 带 TTL 的内存 LRU，并发安全 |
| L2 持久化 | 追加式 `cache.wal`，启动时 replay |
| 缓存键 | 方法、规范化路径、排序 query、请求体 hash 和 `-vary` 组成的确定性 SHA-256 |
| 生产能力 | 请求体限制、超时、`log/slog`、优雅关闭 |

## 管理 API

```bash
curl http://127.0.0.1:8080/__mockbird/cache
curl -X DELETE http://127.0.0.1:8080/__mockbird/cache
curl -X DELETE http://127.0.0.1:8080/__mockbird/cache/{key}
```

## 测试

```bash
go test ./...
go test -race ./...
go build ./cmd/mockbird
```

完整文档请阅读 [README.md](README.md)。
