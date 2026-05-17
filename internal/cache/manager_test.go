package cache

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

type memoryStore struct {
	entries map[string]Entry
}

func newMemoryStore() *memoryStore {
	return &memoryStore{entries: map[string]Entry{}}
}

func (s *memoryStore) Load(key string) (Entry, error) {
	entry, ok := s.entries[key]
	if !ok {
		return Entry{}, errors.New("missing")
	}
	return entry.Clone(), nil
}

func (s *memoryStore) Save(key string, entry Entry) error {
	s.entries[key] = entry.Clone()
	return nil
}

func (s *memoryStore) Delete(key string) error {
	delete(s.entries, key)
	return nil
}

func (s *memoryStore) Clear() error {
	s.entries = map[string]Entry{}
	return nil
}

func (s *memoryStore) ListKeys() ([]string, error) {
	keys := make([]string, 0, len(s.entries))
	for key := range s.entries {
		keys = append(keys, key)
	}
	return keys, nil
}

func TestManagerExpiresEntries(t *testing.T) {
	store := newMemoryStore()
	manager, err := NewManager(Options{MaxEntries: 10, TTL: time.Hour, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	key := Key(http.MethodGet, "/expired", "", nil)
	if err := manager.Set(key, Entry{StatusCode: 200, Headers: http.Header{}, Body: []byte("old"), CreatedAt: time.Now().Add(-2 * time.Hour)}); err != nil {
		t.Fatal(err)
	}

	if _, ok := manager.Get(key); ok {
		t.Fatal("expected expired entry to be ignored")
	}
	if _, ok := store.entries[key]; ok {
		t.Fatal("expected expired entry to be removed from disk store")
	}
}

func TestManagerEvictsRAM(t *testing.T) {
	store := newMemoryStore()
	manager, err := NewManager(Options{MaxEntries: 1, TTL: time.Hour, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	first := Key(http.MethodGet, "/first", "", nil)
	second := Key(http.MethodGet, "/second", "", nil)

	_ = manager.Set(first, Entry{StatusCode: 200, Headers: http.Header{}, Body: []byte("first"), CreatedAt: time.Now()})
	_ = manager.Set(second, Entry{StatusCode: 200, Headers: http.Header{}, Body: []byte("second"), CreatedAt: time.Now()})

	if manager.Len() != 1 {
		t.Fatalf("expected one RAM entry, got %d", manager.Len())
	}
	if _, ok := store.entries[first]; !ok {
		t.Fatal("expected evicted RAM entry to remain on disk store")
	}
}
