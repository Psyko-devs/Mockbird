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
	"os"
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
	Target      *url.URL
	Cache       Cache
	Client      *http.Client
	Pretty      bool
	VaryHeaders []string
	MaxBody     int64
	MaxResponse int64
	Logger      *slog.Logger
}

type Handler struct {
	target      *url.URL
	cache       Cache
	client      *http.Client
	pretty      bool
	varyHeaders []string
	maxBody     int64
	maxResponse int64
	logger      *slog.Logger
	group       singleflight.Group
}

type fetchResult struct {
	entry cache.Entry
}

type UpstreamErrorKind string

const (
	ErrTimeout           UpstreamErrorKind = "timeout"
	ErrDNS               UpstreamErrorKind = "dns_error"
	ErrTLS               UpstreamErrorKind = "tls_error"
	ErrConnectionRefused UpstreamErrorKind = "connection_refused"
	ErrCanceled          UpstreamErrorKind = "context_canceled"
	ErrUpstream          UpstreamErrorKind = "upstream_error"
)

type UpstreamError struct {
	Kind UpstreamErrorKind
	Err  error
}

func (e *UpstreamError) Error() string {
	return e.Err.Error()
}

func (e *UpstreamError) Unwrap() error {
	return e.Err
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
	maxBody := opts.MaxBody
	if maxBody <= 0 {
		maxBody = 10 << 20
	}
	maxResponse := opts.MaxResponse
	if maxResponse <= 0 {
		maxResponse = 100 << 20
	}
	return &Handler{
		target:      opts.Target,
		cache:       opts.Cache,
		client:      client,
		pretty:      opts.Pretty,
		varyHeaders: append([]string(nil), opts.VaryHeaders...),
		maxBody:     maxBody,
		maxResponse: maxResponse,
		logger:      logger,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/favicon.ico" {
		http.NotFound(w, r)
		return
	}

	body, err := h.readBody(w, r)
	if err != nil {
		h.writeError(w, http.StatusRequestEntityTooLarge, "request_body_too_large", err)
		return
	}

	key := cache.Key(cache.KeyRequest{
		Method:      r.Method,
		Path:        r.URL.EscapedPath(),
		RawQuery:    r.URL.RawQuery,
		Body:        body,
		Headers:     r.Header,
		VaryHeaders: h.varyHeaders,
	})
	if entry, ok := h.cache.Get(key); ok {
		h.logger.Debug("cache hit", "key", key, "method", r.Method, "path", r.URL.Path)
		writeEntry(w, entry, h.pretty)
		return
	}

	value, err, _ := h.group.Do(key, func() (any, error) {
		if entry, ok := h.cache.Get(key); ok {
			return fetchResult{entry: entry}, nil
		}
		return h.fetchAndMaybeCache(r, body, key)
	})
	if err != nil {
		upstreamErr := classifyError(err)
		h.logger.Error("upstream request failed", "key", key, "method", r.Method, "path", r.URL.Path, "kind", upstreamErr.Kind, "error", err)
		writeJSONError(w, http.StatusBadGateway, string(upstreamErr.Kind), upstreamErr.Error())
		return
	}

	result := value.(fetchResult)
	if result.entry.StatusCode != 0 {
		writeEntry(w, result.entry, h.pretty)
	}
}

func (h *Handler) fetchAndMaybeCache(original *http.Request, body []byte, key string) (fetchResult, error) {
	upstreamURL := h.upstreamURL(original.URL)
	req, err := http.NewRequestWithContext(original.Context(), original.Method, upstreamURL.String(), bytes.NewReader(body))
	if err != nil {
		return fetchResult{}, err
	}
	copyRequestHeader(req.Header, original.Header)
	req.Host = h.target.Host

	resp, err := h.client.Do(req)
	if err != nil {
		return fetchResult{}, err
	}
	defer resp.Body.Close()

	respBody, exceeded, err := readResponseBody(resp.Body, h.maxResponse)
	if err != nil {
		return fetchResult{}, err
	}
	if exceeded {
		return fetchResult{}, fmt.Errorf("upstream response exceeds max response size %d", h.maxResponse)
	}

	entry := cache.Entry{
		StatusCode: resp.StatusCode,
		Headers:    responseHeaders(resp),
		Body:       respBody,
		CreatedAt:  time.Now().UTC(),
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

	return fetchResult{entry: entry}, nil
}

func (h *Handler) upstreamURL(incoming *url.URL) *url.URL {
	out := *h.target
	out.Path = joinPaths(h.target.Path, incoming.Path)
	out.RawPath = joinEscapedPaths(h.target.EscapedPath(), incoming.EscapedPath())
	if (&url.URL{Path: out.Path}).EscapedPath() == out.RawPath {
		out.RawPath = ""
	}
	out.RawQuery = mergeQueries(h.target.RawQuery, incoming.RawQuery)
	out.Fragment = ""
	return &out
}

func (h *Handler) readBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	defer r.Body.Close()

	limited := http.MaxBytesReader(w, r.Body, h.maxBody)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func writeEntry(w http.ResponseWriter, entry cache.Entry, pretty bool) {
	headers := w.Header()
	copyResponseHeader(headers, entry.Headers)

	body := entry.Body
	if pretty {
		body = prettyJSONIfPossible(entry.Headers, entry.Body)
	}
	headers.Set("Content-Length", fmt.Sprintf("%d", len(body)))
	w.WriteHeader(entry.StatusCode)
	_, _ = w.Write(body)
}

func readResponseBody(r io.Reader, limit int64) ([]byte, bool, error) {
	var buf bytes.Buffer
	n, err := io.CopyN(&buf, r, limit+1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, false, err
	}
	if n > limit {
		return buf.Bytes(), true, nil
	}
	return buf.Bytes(), false, nil
}

func copyRequestHeader(dst, src http.Header) {
	for key, values := range src {
		if hopByHopHeader(key) {
			continue
		}
		dst.Del(key)
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func copyResponseHeader(dst, src http.Header) {
	for key, values := range src {
		if hopByHopHeader(key) || strings.EqualFold(key, "Transfer-Encoding") {
			continue
		}
		dst.Del(key)
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func responseHeaders(resp *http.Response) http.Header {
	headers := resp.Header.Clone()
	if resp.ContentLength >= 0 {
		headers.Set("Content-Length", fmt.Sprintf("%d", resp.ContentLength))
	} else {
		headers.Del("Content-Length")
	}
	return headers
}

func hopByHopHeader(key string) bool {
	switch strings.ToLower(key) {
	case "connection", "proxy-connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailer", "transfer-encoding", "upgrade":
		return true
	default:
		return false
	}
}

func isCacheable(status int) bool {
	return status >= 200 && status < 400
}

func prettyJSONIfPossible(headers http.Header, body []byte) []byte {
	if headers.Get("Content-Encoding") != "" || !strings.Contains(strings.ToLower(headers.Get("Content-Type")), "json") || !json.Valid(body) {
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

func joinPaths(base, incoming string) string {
	if base == "" || base == "/" {
		if incoming == "" {
			return "/"
		}
		return incoming
	}
	if incoming == "" || incoming == "/" {
		return base
	}
	switch {
	case strings.HasSuffix(base, "/") && strings.HasPrefix(incoming, "/"):
		return base + strings.TrimPrefix(incoming, "/")
	case !strings.HasSuffix(base, "/") && !strings.HasPrefix(incoming, "/"):
		return base + "/" + incoming
	default:
		return base + incoming
	}
}

func joinEscapedPaths(base, incoming string) string {
	if base == "" || base == "/" {
		if incoming == "" {
			return "/"
		}
		return incoming
	}
	if incoming == "" || incoming == "/" {
		return base
	}
	switch {
	case strings.HasSuffix(base, "/") && strings.HasPrefix(incoming, "/"):
		return base + strings.TrimPrefix(incoming, "/")
	case !strings.HasSuffix(base, "/") && !strings.HasPrefix(incoming, "/"):
		return base + "/" + incoming
	default:
		return base + incoming
	}
}

func mergeQueries(targetRaw, incomingRaw string) string {
	values := url.Values{}
	addQuery(values, targetRaw)
	addQuery(values, incomingRaw)
	return values.Encode()
}

func addQuery(dst url.Values, raw string) {
	if raw == "" {
		return
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return
	}
	for key, list := range values {
		for _, value := range list {
			dst.Add(key, value)
		}
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

func classifyError(err error) *UpstreamError {
	if errors.Is(err, context.Canceled) {
		return &UpstreamError{Kind: ErrCanceled, Err: err}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &UpstreamError{Kind: ErrTimeout, Err: err}
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return &UpstreamError{Kind: ErrTimeout, Err: err}
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return &UpstreamError{Kind: ErrDNS, Err: err}
	}

	var tlsRecordErr tls.RecordHeaderError
	var tlsCertErr *tls.CertificateVerificationError
	if errors.As(err, &tlsRecordErr) || errors.As(err, &tlsCertErr) {
		return &UpstreamError{Kind: ErrTLS, Err: err}
	}

	if errors.Is(err, syscall.ECONNREFUSED) {
		return &UpstreamError{Kind: ErrConnectionRefused, Err: err}
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) && errors.Is(opErr.Err, syscall.ECONNREFUSED) {
		return &UpstreamError{Kind: ErrConnectionRefused, Err: err}
	}
	var pathErr *os.SyscallError
	if errors.As(err, &pathErr) && errors.Is(pathErr.Err, syscall.ECONNREFUSED) {
		return &UpstreamError{Kind: ErrConnectionRefused, Err: err}
	}

	return &UpstreamError{Kind: ErrUpstream, Err: err}
}
