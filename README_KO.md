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
  <strong>결정적 LRU + WAL 캐시를 사용하는 offline-first HTTP 리버스 프록시 레코더.</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/Cache-LRU_%2B_WAL-teal?style=for-the-badge" alt="Cache">
  <img src="https://img.shields.io/badge/Concurrency-Race_Tested-success?style=for-the-badge" alt="Race Tested">
</p>

Mockbird는 애플리케이션과 실제 upstream API 사이에서 응답을 기록합니다. 오프라인, rate limit, 재현 가능한 테스트 상황에서 로컬 캐시로 응답을 재생합니다.

## 빠른 시작

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

애플리케이션은 다음 주소를 사용합니다.

```text
http://127.0.0.1:8080
```

## 기능

| 영역 | 설명 |
| --- | --- |
| HTTP Proxy | 리버스 프록시, 상태 코드와 헤더 보존 |
| L1 Cache | TTL이 있는 메모리 LRU, 동기화된 접근 |
| L2 Persistence | append-only `cache.wal`, 시작 시 replay |
| Cache Keys | method, 정규화 path, 정렬된 query, body hash, `-vary` 기반 결정적 SHA-256 |
| Production | body 제한, timeout, `log/slog`, graceful shutdown |

## 관리 API

```bash
curl http://127.0.0.1:8080/__mockbird/cache
curl -X DELETE http://127.0.0.1:8080/__mockbird/cache
curl -X DELETE http://127.0.0.1:8080/__mockbird/cache/{key}
```

## 테스트

```bash
go test ./...
go test -race ./...
go build ./cmd/mockbird
```

전체 문서는 [README.md](README.md)를 참고하세요.
