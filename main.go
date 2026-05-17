package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Global L1 RAM cache and thread-safe mutex
var (
	ramCache   = make(map[string][]byte)
	cacheMutex sync.RWMutex
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

// formatJSON pretty-prints JSON data with indentation for readability
func formatJSON(data []byte) []byte {
	var obj interface{}

	// Try to unmarshal the JSON
	if err := json.Unmarshal(data, &obj); err != nil {
		// If it fails, return the original data (not valid JSON)
		return data
	}

	// Marshal back with pretty printing (2-space indentation)
	formatted, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		// If formatting fails, return the original data
		return data
	}

	return formatted
}

// extractOriginalPath removes the upstream API path prefix to get the original request path
// e.g., "/api/v2/pokemon/ditto" with target "https://pokeapi.co/api/v2" returns "/pokemon/ditto"
func extractOriginalPath(fullPath string, targetURL *url.URL) string {
	// Extract the path component from the target URL
	targetPath := targetURL.Path

	// If the full path starts with the target path, remove it
	if strings.HasPrefix(fullPath, targetPath) {
		originalPath := strings.TrimPrefix(fullPath, targetPath)
		if originalPath == "" {
			originalPath = "/"
		}
		return originalPath
	}

	// If no prefix matches, return the full path
	return fullPath
}

// saveResponseCache writes the API response to disk (L2) and stores it in RAM cache (L1)
func saveResponseCache(cacheDir, fileName string, data []byte) error {
	filePath := filepath.Join(cacheDir, fileName)

	// Format JSON for readability
	formattedData := formatJSON(data)

	// Write to disk (L2 cache)
	if err := os.WriteFile(filePath, formattedData, 0644); err != nil {
		return err
	}

	// Store in RAM cache (L1) with write lock
	cacheMutex.Lock()
	ramCache[fileName] = formattedData
	cacheMutex.Unlock()

	return nil
}

func main() {
	// Define command-line flags
	targetURL := flag.String("target", "https://jsonplaceholder.typicode.com", "The upstream API you want to mock.")
	port := flag.Int("port", 8080, "The local port to run Mockbird on.")
	cacheDir := flag.String("dir", "./mockbird_cache", "The directory to save the JSON files.")
	flag.Parse()

	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(*cacheDir, 0755); err != nil {
		fmt.Printf("❌ ERROR: Failed to create cache directory %s: %v\n", *cacheDir, err)
		return
	}

	target, err := url.Parse(*targetURL)
	if err != nil {
		fmt.Printf("❌ ERROR: Invalid target URL: %v\n", err)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// Handle errors when the upstream API is unreachable (offline mode)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		fmt.Printf("❌ ERROR: Cannot reach %s. You are offline and no mock cache exists for this route!\n", *targetURL)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"error": "Mockbird is offline, and no local cache file was found for this route."}`))
	}

	// Intercept API responses and intelligently cache them
	proxy.ModifyResponse = func(resp *http.Response) error {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("⚠️  WARNING: Failed to read response body: %v\n", err)
			resp.Body = io.NopCloser(bytes.NewBuffer(body))
			return nil
		}

		// Extract the original request path (remove upstream API prefix)
		originalPath := extractOriginalPath(resp.Request.URL.Path, target)
		fileName := getFileName(resp.Request.Method, originalPath)
		statusCode := resp.StatusCode

		// Smart Response Filtering: Only cache 2xx and 3xx status codes
		if statusCode >= 200 && statusCode < 400 {
			// Success or redirect response - safe to cache
			if err := saveResponseCache(*cacheDir, fileName, body); err != nil {
				fmt.Printf("⚠️  WARNING: Failed to cache response to %s: %v\n", fileName, err)
			} else {
				fmt.Printf("💾 RECORDED: Saved new photocopy to %s!\n", fileName)
			}
		} else {
			// Error response (4xx or 5xx) - do not cache
			fmt.Printf("⚠️  SKIPPED CACHING: Upstream returned status %d for %s %s\n", statusCode, resp.Request.Method, originalPath)
		}

		// Format JSON for readability before returning to client
		formattedBody := formatJSON(body)

		// Return formatted JSON to client
		resp.Body = io.NopCloser(bytes.NewBuffer(formattedBody))

		// Update Content-Length since we modified the body
		resp.ContentLength = int64(len(formattedBody))
		resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(formattedBody)))

		return nil
	}

	// Main request handler: L1/L2 cache check, then proxy to upstream API
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Block favicon requests to keep logs clean
		if r.URL.Path == "/favicon.ico" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		fileName := getFileName(r.Method, r.URL.Path)
		filePath := filepath.Join(*cacheDir, fileName)

		// STEP 1: Check L1 RAM cache with read lock
		cacheMutex.RLock()
		data, exists := ramCache[fileName]
		cacheMutex.RUnlock()

		if exists {
			// L1 RAM cache hit!
			fmt.Printf("🚀 RAM CACHE: Serving %s instantly from memory!\n", fileName)

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Write(data)
			return
		}

		// STEP 2: Check L2 disk cache
		if _, err := os.Stat(filePath); err == nil {
			// File exists on disk - read it
			data, err := os.ReadFile(filePath)
			if err != nil {
				fmt.Printf("⚠️  WARNING: Failed to read cache file %s: %v\n", fileName, err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			// Load into L1 RAM cache with write lock
			cacheMutex.Lock()
			ramCache[fileName] = data
			cacheMutex.Unlock()

			// Serve from disk
			fmt.Printf("⚡ DISK CACHE: Serving %s from local storage!\n", fileName)

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Write(data)
			return
		}

		// STEP 3: No cache found - fetch from real API (Record Mode)
		fmt.Printf("🌐 RECORD MODE: Fetching from real API -> %s %s\n", r.Method, r.URL.Path)

		r.Host = target.Host
		r.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		r.Header.Set("Accept", "application/json, text/plain, */*")
		r.Header.Del("If-None-Match")
		r.Header.Del("If-Modified-Since")
		r.Header.Set("Accept-Encoding", "identity")

		proxy.ServeHTTP(w, r)
	})

	// Display startup banner
	fmt.Println()
	fmt.Println("🦅 Mockbird is alive!")

	// Display server details with spinner
	animatedSpinner("Initializing server", 1*time.Second)
	fmt.Printf("📍 Targeting: %s\n", *targetURL)
	fmt.Printf("🚀 Listening on: http://127.0.0.1:%d\n", *port)
	fmt.Printf("💾 Cache directory: %s\n", *cacheDir)
	fmt.Println("📡 Ready to serve offline. Connect with your app now!")
	fmt.Println()

	listenAddr := fmt.Sprintf("127.0.0.1:%d", *port)
	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		fmt.Printf("❌ ERROR: Failed to start server: %v\n", err)
	}
}
