package proxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/Psyko-devs/mockbird/internal/cache"
	"golang.org/x/sync/singleflight"
)

type Cache interface {
	Get(key string) (cache.Entry, bool)
	Set(key string, entry cache.Entry) error
}

type Options struct {
	Target *url.URL
	Cache  Cache
	Client *http.Client
	Pretty bool
	Logger *slog.Logger
}

type Handler struct {
	target *url.URL
	cache  Cache
	client *http.Client
	pretty bool
	logger *slog.Logger
	group  singleflight.Group
}

type fetchResult struct {
	entry     cache.Entry
	cacheable bool
}

func New(opts Options) *Handler {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	client := opts.Client
	if client == nil {
		client = http.DefaultClient
	}
	return &Handler{
		target: opts.Target,
		cache:  opts.Cache,
		client: client,
		pretty: opts.Pretty,
		logger: logger,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/favicon.ico" {
		http.NotFound(w, r)
		return
	}

	body, err := readBody(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "request_body_error", err)
		return
	}

	key := cache.Key(r.Method, r.URL.Path, r.URL.RawQuery, body)
	if entry, ok := h.cache.Get(key); ok {
		h.logger.Debug("cache hit", "key", key, "method", r.Method, "path", r.URL.Path)
		writeEntry(w, entry)
		return
	}

	value, err, _ := h.group.Do(key, func() (any, error) {
		if entry, ok := h.cache.Get(key); ok {
			return fetchResult{entry: entry, cacheable: true}, nil
		}
		return h.fetchAndMaybeCache(r.Context(), r, body, key)
	})
	if err != nil {
		kind := classifyError(err)
		h.logger.Error("upstream request failed", "key", key, "method", r.Method, "path", r.URL.Path, "kind", kind, "error", err)
		writeJSONError(w, http.StatusBadGateway, kind, err.Error())
		return
	}

	result := value.(fetchResult)
	writeEntry(w, result.entry)
}

func (h *Handler) fetchAndMaybeCache(ctx context.Context, original *http.Request, body []byte, key string) (fetchResult, error) {
	upstreamURL := h.upstreamURL(original.URL)
	req, err := http.NewRequestWithContext(ctx, original.Method, upstreamURL.String(), bytes.NewReader(body))
	if err != nil {
		return fetchResult{}, err
	}
	copyHeader(req.Header, original.Header)
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Del("If-None-Match")
	req.Header.Del("If-Modified-Since")
	req.Host = h.target.Host

	resp, err := h.client.Do(req)
	if err != nil {
		return fetchResult{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fetchResult{}, err
	}

	headers := resp.Header.Clone()
	bodyToStore := respBody
	if h.pretty {
		bodyToStore = prettyJSONIfPossible(headers, respBody)
		if len(bodyToStore) != len(respBody) {
			headers.Set("Content-Length", fmt.Sprintf("%d", len(bodyToStore)))
		}
	}

	entry := cache.Entry{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       bodyToStore,
		CreatedAt:  time.Now(),
	}

	if isCacheable(resp.StatusCode) {
		if err := h.cache.Set(key, entry); err != nil {
			h.logger.Warn("failed to persist cache entry", "key", key, "error", err)
		} else {
			h.logger.Info("recorded upstream response", "key", key, "method", original.Method, "path", original.URL.Path, "status", resp.StatusCode)
		}
	} else {
		h.logger.Debug("skipped non-cacheable upstream response", "key", key, "status", resp.StatusCode)
	}

	return fetchResult{entry: entry, cacheable: isCacheable(resp.StatusCode)}, nil
}

func (h *Handler) upstreamURL(incoming *url.URL) *url.URL {
	out := *h.target
	out.Path = joinPaths(h.target.Path, incoming.Path)
	out.RawPath = ""
	if h.target.RawQuery == "" || incoming.RawQuery == "" {
		out.RawQuery = h.target.RawQuery + incoming.RawQuery
	} else {
		out.RawQuery = h.target.RawQuery + "&" + incoming.RawQuery
	}
	return &out
}

func readBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func writeEntry(w http.ResponseWriter, entry cache.Entry) {
	copyHeader(w.Header(), entry.Headers)
	w.WriteHeader(entry.StatusCode)
	_, _ = w.Write(entry.Body)
}

func copyHeader(dst, src http.Header) {
	for key, values := range src {
		dst.Del(key)
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func isCacheable(status int) bool {
	return status >= 200 && status < 400
}

func prettyJSONIfPossible(headers http.Header, body []byte) []byte {
	if !strings.Contains(strings.ToLower(headers.Get("Content-Type")), "json") || !json.Valid(body) {
		return body
	}
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return body
	}
	formatted, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return body
	}
	return formatted
}

func joinPaths(base, path string) string {
	switch {
	case base == "" || base == "/":
		if path == "" {
			return "/"
		}
		return path
	case path == "" || path == "/":
		return base
	case strings.HasSuffix(base, "/") && strings.HasPrefix(path, "/"):
		return base + strings.TrimPrefix(path, "/")
	case !strings.HasSuffix(base, "/") && !strings.HasPrefix(path, "/"):
		return base + "/" + path
	default:
		return base + path
	}
}

func (h *Handler) writeError(w http.ResponseWriter, status int, kind string, err error) {
	h.logger.Warn("request failed", "kind", kind, "error", err)
	writeJSONError(w, status, kind, err.Error())
}

func writeJSONError(w http.ResponseWriter, status int, kind, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   message,
		"type":    kind,
		"message": "upstream request failed and no valid cache entry was available",
	})
}

func classifyError(err error) string {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "dns_error"
	}

	var tlsErr tls.RecordHeaderError
	if errors.As(err, &tlsErr) {
		return "tls_error"
	}

	if errors.Is(err, syscall.ECONNREFUSED) || strings.Contains(strings.ToLower(err.Error()), "connection refused") {
		return "connection_refused"
	}

	return "upstream_error"
}
