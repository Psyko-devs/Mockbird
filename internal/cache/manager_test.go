package cache

import (
	"net/http"
	"sync"
	"testing"
	"time"
)

type memoryStore struct {
	mu      sync.Mutex
	entries map[string]Entry
}

func newMemoryStore() *memoryStore {
	return &memoryStore{entries: map[string]Entry{}}
}

func (s *memoryStore) Save(key string, entry Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[key] = entry.DeepCopy()
	return nil
}

func (s *memoryStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, key)
	return nil
}

func (s *memoryStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = map[string]Entry{}
	return nil
}

func (s *memoryStore) Snapshot() ([]SnapshotEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]SnapshotEntry, 0, len(s.entries))
	for key, entry := range s.entries {
		out = append(out, SnapshotEntry{
			Key:        key,
			StatusCode: entry.StatusCode,
			Headers:    HTTPHeader(entry.Headers),
			BodySize:   len(entry.Body),
			CreatedAt:  entry.CreatedAt,
			Source:     "disk",
		})
	}
	return out, nil
}

func (s *memoryStore) Restore() ([]StoredEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]StoredEntry, 0, len(s.entries))
	for key, entry := range s.entries {
		out = append(out, StoredEntry{Key: key, Entry: entry.DeepCopy()})
	}
	return out, nil
}

func TestManagerExpiresEntries(t *testing.T) {
	store := newMemoryStore()
	manager, err := NewManager(Options{MaxEntries: 10, TTL: time.Hour, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	key := Key(KeyRequest{Method: http.MethodGet, Path: "/expired"})
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
	first := Key(KeyRequest{Method: http.MethodGet, Path: "/first"})
	second := Key(KeyRequest{Method: http.MethodGet, Path: "/second"})

	_ = manager.Set(first, Entry{StatusCode: 200, Headers: http.Header{}, Body: []byte("first"), CreatedAt: time.Now()})
	_ = manager.Set(second, Entry{StatusCode: 200, Headers: http.Header{}, Body: []byte("second"), CreatedAt: time.Now()})

	if manager.Len() != 1 {
		t.Fatalf("expected one RAM entry, got %d", manager.Len())
	}
	if _, ok := store.entries[first]; ok {
		t.Fatal("expected evicted RAM entry to be removed from disk store")
	}
	if _, ok := store.entries[second]; !ok {
		t.Fatal("expected newest entry to remain in disk store")
	}
}

func TestManagerConcurrentAccess(t *testing.T) {
	store := newMemoryStore()
	manager, err := NewManager(Options{MaxEntries: 25, TTL: time.Hour, Store: store})
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := Key(KeyRequest{Method: http.MethodGet, Path: "/load/" + string(rune('a'+i%26))})
			_ = manager.Set(key, Entry{StatusCode: 200, Headers: http.Header{}, Body: []byte("ok"), CreatedAt: time.Now()})
			_, _ = manager.Get(key)
			if i%5 == 0 {
				_ = manager.Delete(key)
			}
		}()
	}
	wg.Wait()

	if _, err := manager.Snapshot(); err != nil {
		t.Fatal(err)
	}
}
