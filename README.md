# 🦅 Mockbird - Offline-First Mock API Server

> A production-ready, zero-config reverse proxy with intelligent dual-layer caching for seamless offline API development. Built in Go with thread-safe operations and professional terminal UX.

[![Go Version](https://img.shields.io/badge/go-1.25.0-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green?style=for-the-badge)](LICENSE)
[![Offline First](https://img.shields.io/badge/Offline-First-success?style=for-the-badge)](https://offlinefirst.org)

## 📖 Table of Contents
- [Overview](#overview)
- [The Problem](#-the-problem)
- [The Solution](#-the-solution)
- [Key Features](#%EF%B8%8F-key-features)
- [Architecture](#%EF%B8%8F-architecture)
- [Installation](#-installation)
- [Quick Start](#-quick-start)
- [CLI Configuration](#%EF%B8%8F-cli-configuration)
- [Usage Examples](#-usage-examples)
- [How It Works](#-how-it-works)
- [Technical Details](#-technical-details)
- [FAQ](#-faq)

---

## Overview

**Mockbird** is a lightweight, production-ready mock API server that sits between your application and the real API. It intelligently caches API responses locally, enabling you to:

- **Develop offline** without internet connectivity
- **Eliminate API rate limits** during development
- **Reduce external API costs** through smart caching
- **Speed up tests** with instant cached responses (1-5ms latency)
- **Mock any REST API** without complex setup

Perfect for frontend development, automated testing, and offline-first applications.

---

## 🤔 The Problem

Developing modern applications against live third-party APIs is challenging:

- ⚠️ **No internet = no development** — WiFi down? Project halted.
- 💰 **Rate limiting costs** — Hammer a test endpoint, hit limits.
- 🐌 **Slow development cycles** — Waiting for real API responses kills productivity.
- 🔌 **API dependency** — External services down? Your team can't work.
- 📊 **Expensive quota usage** — Every request counts against your API quota.

---

## 💡 The Solution

Mockbird acts as an intelligent local proxy with **dual-layer caching**:

```
┌─────────────────┐
│  Your App       │
└────────┬────────┘
         │ Request to localhost:8080
         ▼
    ┌─────────────────────┐
    │  Mockbird Proxy     │
    │  ┌───────────────┐  │
    │  │ L1 RAM Cache  │◄─┼── Ultra-fast (🚀 instant)
    │  └───────────────┘  │
    │  ┌───────────────┐  │
    │  │ L2 Disk Cache │◄─┼── Persistent (⚡ fast)
    │  └───────────────┘  │
    └──────────┬──────────┘
               │ Cache miss? Fetch from real API
               ▼
         ┌──────────────┐
         │  Real API    │
         │ Any REST API │
         └──────────────┘
```

**Three operating modes:**

1. **🚀 L1 RAM Cache Hit** - Instant response from memory (~1ms)
2. **⚡ L2 Disk Cache Hit** - Response from local storage (loads into L1)
3. **🌐 Record Mode** - Fetches from real API, saves to L1+L2 automatically

---

## ⚙️ Key Features

### Core Functionality
- ✅ **Dual-layer caching**: L1 RAM (ultra-fast) + L2 Disk (persistent)
- ✅ **Thread-safe operations**: Safe concurrent access with `sync.RWMutex`
- ✅ **Smart response filtering**: Only caches 2xx/3xx, skips 4xx/5xx errors
- ✅ **Auto-routing**: Generates cache filenames from HTTP method + path
- ✅ **JSON auto-formatting**: Readable 2-space indented cache files

### Configuration & Flexibility
- ✅ **CLI flags**: `--target`, `--port`, `--dir` for full customization
- ✅ **Custom upstream APIs**: Works with any REST API
- ✅ **Auto-directory creation**: Automatically creates cache directories
- ✅ **Path extraction**: Correctly handles API prefixes in cache filenames

### Developer Experience
- ✅ **Zero configuration**: Run and it just works
- ✅ **Professional logging**: Color-coded status messages with emojis
- ✅ **Terminal animations**: Spinner during startup + typewriter effects
- ✅ **CORS support**: Automatic `Access-Control-Allow-Origin: *` headers
- ✅ **Graceful error handling**: Standard JSON errors when offline

### Security & Reliability
- ✅ **User-Agent spoofing**: Includes browser headers to bypass bot detection
- ✅ **Cache busting**: Strips `If-None-Match` and `If-Modified-Since` headers
- ✅ **Error resilience**: Doesn't crash on malformed responses
- ✅ **Bulletproof offline mode**: Returns helpful JSON errors when completely offline

---

## 🏗️ Architecture

### File Structure
```
mockbird/
├── main.go                 # Core application (266 lines, production-ready)
├── go.mod                  # Go module definition
└── mockbird_cache/         # Default cache directory (auto-created)
    ├── GET_resource1.json
    ├── GET_resource2.json
    └── POST_resource.json
```

### Caching Architecture

**L1 RAM Cache (In-Memory)**
- Global: `ramCache map[string][]byte`
- Thread-safe: Protected by `sync.RWMutex`
- Fastest access: ~1ms per lookup
- Lost on server restart

**L2 Disk Cache (Persistent Storage)**
- Location: Configurable via `--dir` flag
- Format: JSON files with 2-space indentation
- Survives server restart
- Fallback when L1 is empty

**Record Mode (Real API Fetch)**
- Triggered on cache miss (L1 + L2 both empty)
- Saves response to both L1 and L2
- Only caches 2xx/3xx status codes
- Skips 4xx/5xx error responses

### Request Flow

```
HTTP Request arrives
        │
        ▼
Check if /favicon.ico? ──Yes──> Return 404 (keep logs clean)
        │
       No
        │
        ▼
Extract cache filename from method + path
        │
        ▼
Check L1 RAM cache? ──Hit──> Serve from RAM (🚀)
        │
       Miss
        │
        ▼
Check L2 Disk cache? ──Hit──> Load to L1, serve (⚡)
        │
       Miss
        │
        ▼
Record Mode: Fetch from upstream API (🌐)
        │
        ▼
Response received ──Status 2xx/3xx?──> Save to L1+L2 (💾)
        │                    │
        │                   No
        │                    │
        │               ⚠️ Skip caching
        │                    │
        ▼                    ▼
Format JSON ──────────> Return to client
```

---

## 📦 Installation

### Prerequisites
- Go 1.25.0 or higher
- Git

### Clone & Setup

```bash
# Clone the repository
git clone https://github.com/Psyko-devs/Mockbird.git
cd Mockbird

# Download dependencies
go mod download

# Run the server
go run main.go
```

### Optional: Build Executable

```bash
# Build for current OS
go build -o mockbird

# Run the executable
./mockbird --target https://api.example.com --port 3000 --dir ./api_cache
```

---

## 🚀 Quick Start

### Basic Usage

```bash
# Start with default settings
go run main.go
```

This will:
- 🎯 Target: `https://jsonplaceholder.typicode.com` (default example API)
- 🔊 Listen on: `http://127.0.0.1:8080`
- 💾 Cache to: `./mockbird_cache`

Then in your app:
```javascript
// Instead of calling the real API directly
const response = await fetch('http://127.0.0.1:8080/posts/1');
const data = await response.json();
console.log(data);
```

### Custom Upstream API

```bash
go run main.go \
  --target https://api.example.com/v1 \
  --port 3000 \
  --dir ./my_api_cache
```

First request to `/resource`:
```
🌐 RECORD MODE: Fetching from real API -> GET /resource
💾 RECORDED: Saved new photocopy to GET_resource.json!
```

Second request to `/resource`:
```
🚀 RAM CACHE: Serving GET_resource.json instantly from memory!
```

---

## ⚙️ CLI Configuration

### Available Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--target` | string | `https://jsonplaceholder.typicode.com` | Upstream REST API to cache and mock |
| `--port` | int | `8080` | Local port to listen on |
| `--dir` | string | `./mockbird_cache` | Directory to store cache files |

### Examples

```bash
# Target any public REST API
go run main.go \
  --target https://api.example.com/v1 \
  --port 4000 \
  --dir ./api_cache

# Target another service with path prefix
go run main.go \
  --target https://service.example.com/api/v2 \
  --port 8888 \
  --dir ./service_cache

# Target local backend during testing
go run main.go \
  --target http://localhost:5000 \
  --port 9000 \
  --dir ./local_backend_cache
```

---

## 💡 Usage Examples

### Example 1: Frontend Development

```bash
# Terminal 1: Start Mockbird
cd mockbird
go run main.go \
  --target https://api.example.com \
  --port 3000 \
  --dir ./api_cache
```

```bash
# Terminal 2: Frontend development
cd my-app
npm start
```

```javascript
// Your app component - point to localhost instead of real API
async function fetchData(endpoint) {
  const response = await fetch(`http://localhost:3000${endpoint}`);
  return response.json();
}

// Now works OFFLINE - Mockbird serves cached responses!
```

**First request flow:**
- 🌐 Browser requests → Mockbird
- 🌐 Mockbird fetches from real API
- 💾 Response saved automatically
- 🚀 Browser gets response instantly

**Second request flow (offline):**
- 🌐 Browser requests → Mockbird
- 🚀 Mockbird serves from RAM cache
- ⚡ Browser gets response (no internet needed!)

### Example 2: Automated Testing

```bash
# Start Mockbird with your target API
go run main.go --target https://api.example.com --dir ./test_cache
```

```javascript
// test.js - Jest test file
describe('API Tests', () => {
  beforeAll(() => {
    // Point to mock server
    process.env.API_URL = 'http://127.0.0.1:8080';
  });

  test('should fetch all posts', async () => {
    const response = await fetch('http://127.0.0.1:8080/posts');
    const data = await response.json();
    
    // First run: Real API called, response cached
    // Subsequent runs: 1ms response from cache
    expect(Array.isArray(data)).toBe(true);
  });

  test('should handle offline gracefully', async () => {
    // Kill WiFi - tests still pass from cache!
    const response = await fetch('http://127.0.0.1:8080/posts/1');
    expect(response.status).toBe(200);
  });
});
```

### Example 3: Multi-Environment Configuration

```bash
# Development: Real API with caching
go run main.go \
  --target https://api.production.com \
  --port 8080 \
  --dir ./dev_cache

# CI/CD Pipeline: Isolated cache directory per test run
go run main.go \
  --target https://api.staging.com \
  --port 9000 \
  --dir ./ci_cache_${BUILD_ID}
```

---

## 🔍 How It Works

### Request Interception

When a request arrives at `http://127.0.0.1:8080/posts`:

1. **Generate cache key** from HTTP method + path
   - Method: `GET`
   - Path: `/posts`
   - Cache key: `GET_posts.json`

2. **Check L1 RAM cache** (fastest)
   - Thread-safe read with `cacheMutex.RLock()`
   - If found: Return instantly (~1ms)
   - If miss: Continue

3. **Check L2 Disk cache** (fallback)
   - If found: Load into L1 RAM
   - Return to client
   - If miss: Continue

4. **Record Mode** (fetch and cache)
   - Forward request to upstream API
   - Receive response
   - If 2xx/3xx: Save to disk + RAM
   - If 4xx/5xx: Skip caching
   - Return to client

### Smart Response Filtering

**What gets cached?**
- ✅ 200 OK
- ✅ 201 Created
- ✅ 204 No Content
- ✅ 301/302 Redirects
- ✅ 304 Not Modified

**What does NOT get cached?**
- ❌ 400 Bad Request
- ❌ 401 Unauthorized
- ❌ 403 Forbidden
- ❌ 404 Not Found
- ❌ 500 Internal Server Error
- ❌ 503 Service Unavailable

This prevents caching of error responses that might mislead development.

### Path Extraction for Custom APIs

When using an upstream API with path prefixes (e.g., `--target https://api.example.com/v2`):

```
Upstream Response Path:  /v2/resource/123
                             ▲
                  Prefix to extract
                                
Original Request Path:   /resource/123
Cache Key:               GET_resource_123.json
```

This ensures cache filenames are clean and reusable across different upstream API prefixes.

---

## 🔬 Technical Details

### Thread Safety

All cache operations use `sync.RWMutex`:

```go
// Multiple readers (no lock contention)
cacheMutex.RLock()
data, exists := ramCache[fileName]
cacheMutex.RUnlock()

// Exclusive writer (safe updates)
cacheMutex.Lock()
ramCache[fileName] = data
cacheMutex.Unlock()
```

This allows concurrent read access while preventing race conditions on write.

### JSON Formatting

Cache files are automatically formatted for readability:

```json
{
  "id": 1,
  "title": "sunt aut facere reptat",
  "body": "quia et suscipit..."
}
```

Uses `json.MarshalIndent(data, "", "  ")` for 2-space indentation.

### Content-Length Header Updates

When JSON is formatted, Content-Length is automatically recalculated:

```go
resp.ContentLength = int64(len(formattedBody))
resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(formattedBody)))
```

This prevents client-side truncation issues.

### Reverse Proxy Customization

Custom headers injected for API compatibility:

```go
r.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)...")
r.Header.Set("Accept", "application/json, text/plain, */*")
r.Header.Del("If-None-Match")           // Cache busting
r.Header.Del("If-Modified-Since")       // Cache busting
r.Header.Set("Accept-Encoding", "identity")
```

---

## ❓ FAQ

**Q: What if I go offline with no cached responses?**  
A: Mockbird returns a helpful JSON error:
```json
{
  "error": "Mockbird is offline, and no local cache file was found for this route."
}
```

**Q: Can I use this with GraphQL APIs?**  
A: Currently designed for REST APIs. GraphQL support could be added as a feature.

**Q: Does it cache POST requests?**  
A: Yes! It caches by HTTP method + path, so `POST /users` and `GET /users` are stored separately.

**Q: How do I clear the cache?**  
A: Delete the cache directory:
```bash
rm -rf ./mockbird_cache
```
The directory is auto-recreated on next startup.

**Q: Can I share cache files between developers?**  
A: Yes! Commit the cache directory to Git:
```bash
git add mockbird_cache/
git commit -m "Add API cache for offline development"
```

**Q: What's the memory impact of L1 caching?**  
A: Depends on API response sizes. For typical JSON APIs (10-50KB per endpoint), expect ~1-10MB for L1.

**Q: Can I modify cache files manually?**  
A: Yes! Just edit the JSON files in the cache directory and Mockbird will serve your changes.

---

## 🎓 What This Project Demonstrates

- **Reverse proxy implementation** in Go with `httputil.ReverseProxy`
- **Dual-layer caching** strategy (memory + disk)
- **Thread-safe concurrent access** with sync primitives
- **HTTP request/response manipulation**
- **CLI application design** with flags and configuration
- **Terminal UX** with animations and color formatting
- **Professional error handling** and logging

---

## 📝 License

MIT License - Feel free to use this in your projects!

---

## 🤝 Contributing

Found a bug? Have a feature idea? Feel free to open issues or submit pull requests!

---

## 🦅 Why "Mockbird"?

A mockingbird mimics the sounds of other birds. Mockbird mimics API responses from anywhere. Perfect match! 🎵