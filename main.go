package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"
)

// typeWriter simulates typing animation in the terminal with a smooth character-by-character effect
func typeWriter(text string, delay time.Duration) {
	for _, char := range text {
		fmt.Print(string(char))
		time.Sleep(delay)
	}
	fmt.Println()
}

// animatedSpinner displays a rotating spinner while a message is shown
func animatedSpinner(message string, duration time.Duration) {
	spinFrames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	startTime := time.Now()

	for time.Since(startTime) < duration {
		for _, frame := range spinFrames {
			fmt.Printf("\r%s %s", frame, message)
			time.Sleep(50 * time.Millisecond)
		}
	}
	fmt.Printf("\r✅ %s\n", message)
}

// getFileName constructs the cache filename from HTTP method and URL path
func getFileName(method, path string) string {
	pathName := strings.ReplaceAll(strings.TrimPrefix(path, "/"), "/", "_")
	if pathName == "" {
		pathName = "index"
	}
	return method + "_" + pathName + ".json"
}

// saveResponseCache writes the API response to a local JSON file for future offline use
func saveResponseCache(fileName string, data []byte) error {
	return os.WriteFile(fileName, data, 0644)
}

func main() {
	const targetURL = "https://jsonplaceholder.typicode.com"

	target, err := url.Parse(targetURL)
	if err != nil {
		fmt.Printf("❌ ERROR: Invalid target URL: %v\n", err)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// Handle errors when the upstream API is unreachable (offline mode)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		fmt.Printf("❌ ERROR: Cannot reach %s. You are offline and no mock cache exists for this route!\n", targetURL)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"error": "Mockbird is offline, and no local cache file was found for this route."}`))
	}

	// Intercept successful API responses and save them as cached JSON files
	proxy.ModifyResponse = func(resp *http.Response) error {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("⚠️  WARNING: Failed to read response body: %v\n", err)
			resp.Body = io.NopCloser(bytes.NewBuffer(body))
			return nil
		}

		fileName := getFileName(resp.Request.Method, resp.Request.URL.Path)

		if err := saveResponseCache(fileName, body); err != nil {
			fmt.Printf("⚠️  WARNING: Failed to cache response to %s: %v\n", fileName, err)
		} else {
			fmt.Printf("💾 RECORDED: Saved new photocopy to %s!\n", fileName)
		}

		resp.Body = io.NopCloser(bytes.NewBuffer(body))
		return nil
	}

	// Main request handler: serves from cache or fetches from API
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Block favicon requests to keep logs clean
		if r.URL.Path == "/favicon.ico" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		fileName := getFileName(r.Method, r.URL.Path)

		// Check if we have a cached response for this request
		if _, err := os.Stat(fileName); err == nil {
			// Animated mock mode message
			fmt.Printf("\r⚡ MOCK MODE: Serving %s instantly from local cache!\n", fileName)

			data, err := os.ReadFile(fileName)
			if err != nil {
				fmt.Printf("\r⚠️  WARNING: Failed to read cache file %s: %v\n", fileName, err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Write(data)
			return
		}

		// No cache found: fetch from real API and save for future offline use
		fmt.Printf("\r🌐 RECORD MODE: Fetching from real API -> %s %s\n", r.Method, r.URL.Path)

		r.Host = target.Host
		r.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		r.Header.Set("Accept", "application/json, text/plain, */*")
		r.Header.Del("If-None-Match")
		r.Header.Del("If-Modified-Since")
		r.Header.Set("Accept-Encoding", "identity")

		proxy.ServeHTTP(w, r)
	})

	// Display startup banner with animated bird
	fmt.Println()
	fmt.Println("🦅 Mockbird is alive!")

	// Display server details with spinner
	animatedSpinner("Initializing server", 1*time.Second)
	fmt.Printf("📍 Targeting: %s\n", targetURL)
	fmt.Println("🚀 Listening on: http://127.0.0.1:8080")
	fmt.Println("📡 Ready to serve offline. Connect with your app now!")
	fmt.Println()

	if err := http.ListenAndServe("127.0.0.1:8080", nil); err != nil {
		fmt.Printf("❌ ERROR: Failed to start server: %v\n", err)
	}
}
