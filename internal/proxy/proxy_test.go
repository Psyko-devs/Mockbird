package proxy

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Psyko-devs/mockbird/internal/cache"
	"github.com/Psyko-devs/mockbird/internal/storage"
)

func newTestHandler(t *testing.T, upstream http.HandlerFunc) (*Handler, *cache.Manager) {
	t.Helper()
	target := httptest.NewServer(upstream)
	t.Cleanup(target.Close)

	targetURL := mustParseURL(t, target.URL)
	store, err := storage.NewDiskStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	manager, err := cache.NewManager(cache.Options{MaxEntries: 2, TTL: time.Hour, Store: store})
	if err != nil {
		t.Fatal(err)
	}

	return New(Options{
		Target: targetURL,
		Cache:  manager,
		Client: target.Client(),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}), manager
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestProxyReplaysHeadersAndStatusCodes(t *testing.T) {
	handler, _ := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Location", "/created/1")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/items", nil))
	if rec.Code != http.StatusCreated {
		t.Fatalf("first status = %d, want %d", rec.Code, http.StatusCreated)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/items", nil))
	if rec.Code != http.StatusCreated {
		t.Fatalf("cached status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if got := rec.Header().Get("Location"); got != "/created/1" {
		t.Fatalf("location = %q, want /created/1", got)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/plain" {
		t.Fatalf("content-type = %q, want text/plain", got)
	}
}

func TestProxyServesOfflineFromCache(t *testing.T) {
	var calls atomic.Int32
	handler, _ := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte("cached"))
	})

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/offline", nil))

	handler.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, io.ErrUnexpectedEOF
	})}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/offline", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "cached" {
		t.Fatalf("body = %q, want cached", rec.Body.String())
	}
	if calls.Load() != 1 {
		t.Fatalf("upstream calls = %d, want 1", calls.Load())
	}
}

func TestProxyDeduplicatesConcurrentMisses(t *testing.T) {
	var calls atomic.Int32
	handler, _ := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte("ok"))
	})

	done := make(chan struct{}, 10)
	for i := 0; i < 10; i++ {
		go func() {
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/same", nil))
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	if calls.Load() != 1 {
		t.Fatalf("upstream calls = %d, want 1", calls.Load())
	}
}

func TestProxyCacheInvalidation(t *testing.T) {
	var calls atomic.Int32
	handler, manager := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte{byte('0' + calls.Add(1))})
	})

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/invalidate", nil))
	key := cache.Key(http.MethodGet, "/invalidate", "", nil)
	if err := manager.Delete(key); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/invalidate", nil))
	if rec.Body.String() != "2" {
		t.Fatalf("body = %q, want 2 after invalidation", rec.Body.String())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
