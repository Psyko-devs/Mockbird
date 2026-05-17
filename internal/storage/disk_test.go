package storage

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Psyko-devs/mockbird/internal/cache"
)

func TestDiskStorePersistsFullEntry(t *testing.T) {
	store, err := NewDiskStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	key := cache.Key(cache.KeyRequest{Method: http.MethodGet, Path: "/resource", RawQuery: "id=1"})
	want := cache.Entry{
		StatusCode: http.StatusCreated,
		Headers: http.Header{
			"Content-Type":  []string{"application/json"},
			"ETag":          []string{`"abc"`},
			"Cache-Control": []string{"max-age=60"},
		},
		Body:      []byte(`{"ok":true}`),
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}

	if err := store.Save(key, want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load(key)
	if err != nil {
		t.Fatal(err)
	}

	if got.StatusCode != want.StatusCode {
		t.Fatalf("status code = %d, want %d", got.StatusCode, want.StatusCode)
	}
	if got.Headers.Get("ETag") != want.Headers.Get("ETag") {
		t.Fatalf("etag = %q, want %q", got.Headers.Get("ETag"), want.Headers.Get("ETag"))
	}
	if string(got.Body) != string(want.Body) {
		t.Fatalf("body = %q, want %q", got.Body, want.Body)
	}
	if !got.CreatedAt.Equal(want.CreatedAt) {
		t.Fatalf("created_at = %s, want %s", got.CreatedAt, want.CreatedAt)
	}
}

func TestDiskStoreDetectsCorruption(t *testing.T) {
	dir := t.TempDir()
	store, err := NewDiskStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	key := cache.Key(cache.KeyRequest{Method: http.MethodGet, Path: "/corrupt"})
	entry := cache.Entry{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": []string{"text/plain"}},
		Body:       []byte("original"),
		CreatedAt:  time.Now().UTC(),
	}
	if err := store.Save(key, entry); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, key+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data[:len(data)-2], []byte(`xx"}`)...)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := store.Load(key); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("Load error = %v, want ErrCorrupt", err)
	}
}

func TestDiskStoreSnapshotUsesIndex(t *testing.T) {
	store, err := NewDiskStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	key := cache.Key(cache.KeyRequest{Method: http.MethodGet, Path: "/snapshot"})
	if err := store.Save(key, cache.Entry{StatusCode: 200, Headers: http.Header{}, Body: []byte("ok"), CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}

	entries, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Key != key || entries[0].BodySize != 2 {
		t.Fatalf("snapshot = %#v, want indexed metadata for %s", entries, key)
	}
}
