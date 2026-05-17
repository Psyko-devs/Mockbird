package storage

import (
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Psyko-devs/mockbird/internal/cache"
)

func TestDiskStoreReplaysWALSet(t *testing.T) {
	dir := t.TempDir()
	store, err := NewDiskStore(dir)
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

	recovered, err := NewDiskStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := recovered.Restore()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("restored entries = %d, want 1", len(entries))
	}
	got := entries[0].Entry
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

func TestDiskStoreReplayDeleteOverridesSet(t *testing.T) {
	dir := t.TempDir()
	store, err := NewDiskStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	key := cache.Key(cache.KeyRequest{Method: http.MethodGet, Path: "/delete"})
	if err := store.Save(key, cache.Entry{StatusCode: 200, Headers: http.Header{}, Body: []byte("ok"), CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(key); err != nil {
		t.Fatal(err)
	}

	recovered, err := NewDiskStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := recovered.Restore()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("restored entries = %d, want 0 after delete", len(entries))
	}
}

func TestDiskStoreReplayClearWipesEntries(t *testing.T) {
	dir := t.TempDir()
	store, err := NewDiskStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/a", "/b"} {
		key := cache.Key(cache.KeyRequest{Method: http.MethodGet, Path: path})
		if err := store.Save(key, cache.Entry{StatusCode: 200, Headers: http.Header{}, Body: []byte(path), CreatedAt: time.Now().UTC()}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Clear(); err != nil {
		t.Fatal(err)
	}

	recovered, err := NewDiskStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := recovered.Restore()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("restored entries = %d, want 0 after clear", len(entries))
	}
}

func TestDiskStoreIgnoresCorruptedTail(t *testing.T) {
	dir := t.TempDir()
	store, err := NewDiskStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	key := cache.Key(cache.KeyRequest{Method: http.MethodGet, Path: "/tail"})
	if err := store.Save(key, cache.Entry{StatusCode: 200, Headers: http.Header{}, Body: []byte("ok"), CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}

	walPath := filepath.Join(dir, walName)
	file, err := os.OpenFile(walPath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(`{"op":"set","key":"not-finished"`); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	recovered, err := NewDiskStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := recovered.Restore()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Key != key {
		t.Fatalf("restored entries = %#v, want original entry only", entries)
	}
}

func TestDiskStoreSnapshotUsesMemoryOnly(t *testing.T) {
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
		t.Fatalf("snapshot = %#v, want memory metadata for %s", entries, key)
	}
}

func TestDiskStoreCompactionKeepsLatestState(t *testing.T) {
	dir := t.TempDir()
	store, err := NewDiskStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	key := cache.Key(cache.KeyRequest{Method: http.MethodGet, Path: "/compact"})
	if err := store.Save(key, cache.Entry{StatusCode: 200, Headers: http.Header{}, Body: []byte("old"), CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(key, cache.Entry{StatusCode: 201, Headers: http.Header{}, Body: []byte("new"), CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if err := store.Compact(); err != nil {
		t.Fatal(err)
	}

	recovered, err := NewDiskStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := recovered.Restore()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("restored entries = %d, want 1", len(entries))
	}
	if entries[0].Entry.StatusCode != 201 || string(entries[0].Entry.Body) != "new" {
		t.Fatalf("restored entry = %#v, want latest compacted state", entries[0].Entry)
	}
}

func TestDiskStoreCreatesOnlyWALOnWritePath(t *testing.T) {
	dir := t.TempDir()
	store, err := NewDiskStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	key := cache.Key(cache.KeyRequest{Method: http.MethodGet, Path: "/wal-only"})
	if err := store.Save(key, cache.Entry{StatusCode: 200, Headers: http.Header{}, Body: []byte("ok"), CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != walName {
			t.Fatalf("unexpected write-path file %q; want only %s", entry.Name(), walName)
		}
	}
}

func TestDiskStoreConcurrentMutations(t *testing.T) {
	store, err := NewDiskStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := cache.Key(cache.KeyRequest{Method: http.MethodGet, Path: "/concurrent/" + string(rune('a'+i%26))})
			_ = store.Save(key, cache.Entry{StatusCode: 200, Headers: http.Header{}, Body: []byte("ok"), CreatedAt: time.Now().UTC()})
			if i%3 == 0 {
				_ = store.Delete(key)
			}
		}()
	}
	wg.Wait()

	if _, err := store.Snapshot(); err != nil {
		t.Fatal(err)
	}
}
