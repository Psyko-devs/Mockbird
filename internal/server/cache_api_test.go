package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Psyko-devs/mockbird/internal/cache"
	"github.com/Psyko-devs/mockbird/internal/storage"
)

func TestCacheAPIInspectClearAndDelete(t *testing.T) {
	store, err := storage.NewDiskStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	manager, err := cache.NewManager(cache.Options{MaxEntries: 10, TTL: time.Hour, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	key := cache.Key(http.MethodGet, "/cached", "", nil)
	if err := manager.Set(key, cache.Entry{StatusCode: 200, Headers: http.Header{"ETag": []string{"abc"}}, Body: []byte("ok"), CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	RegisterCacheRoutes(mux, manager, slog.New(slog.NewTextHandler(io.Discard, nil)))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/__mockbird/cache", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("inspect status = %d, want 200", rec.Code)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/__mockbird/cache/"+key, nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", rec.Code)
	}
	if _, ok := manager.Get(key); ok {
		t.Fatal("expected deleted entry to be absent")
	}

	if err := manager.Set(key, cache.Entry{StatusCode: 200, Headers: http.Header{}, Body: []byte("ok"), CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/__mockbird/cache", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("clear status = %d, want 204", rec.Code)
	}
	if _, ok := manager.Get(key); ok {
		t.Fatal("expected cleared entry to be absent")
	}
}
