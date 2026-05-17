package storage

import (
	"net/http"
	"testing"
	"time"

	"github.com/Psyko-devs/mockbird/internal/cache"
)

func TestDiskStorePersistsFullEntry(t *testing.T) {
	store, err := NewDiskStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	key := cache.Key(http.MethodGet, "/resource", "id=1", nil)
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
