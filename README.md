# 🦅 API Mockingbird

> A zero-config, lightning-fast reverse proxy that caches real API responses locally for seamless offline development. Built in Go.

![Go](https://img.shields.io/badge/go-%2300ADD8.svg?style=for-the-badge&logo=go&logoColor=white)
![Offline](https://img.shields.io/badge/Offline-First-success?style=for-the-badge)

## 🤔 The Problem
Developing frontend applications or automated tests against live third-party APIs can be slow, costly (rate limits), and impossible without an internet connection. 

## 💡 The Solution
Mockingbird sits between your application and the internet. 
1. **Record Mode:** It forwards your request to the real API and secretly saves a copy of the JSON response to your local hard drive.
2. **Mock Mode:** If you go offline or hit the same route again, Mockingbird intercepts the request and instantly serves the local JSON file. 0ms latency. Zero API costs.

## 🚀 Quick Start

1. Clone the repository and run the tool:
\`\`\`bash
go run main.go
\`\`\`

2. Point your frontend application to `http://127.0.0.1:8080` instead of the real API.

3. Turn off your Wi-Fi. Keep coding.

## ⚙️ Features
* **Zero-Configuration:** No massive UI or config files required. It just works.
* **Auto-Routing:** Dynamically generates cache filenames based on the HTTP Method and URL path (e.g., `GET_users.json`).
* **Cloudflare/Bot Bypass:** Injects standard browser `User-Agent` headers to prevent 403 blocks from strict APIs.
* **Cache Busting:** Automatically strips `If-None-Match` headers to ensure fresh data capture.
* **Bulletproof Offline Failsafe:** Gracefully returns standard JSON error objects if an un-cached route is hit while completely offline.

## 🧠 What I Learned Building This
* Low-level HTTP request/response manipulation in Go.
* Implementing and customizing Go's `httputil.ReverseProxy`.
* Bypassing aggressive browser caching and CDN bot protection.
* Building premium CLI interfaces with terminal animations.