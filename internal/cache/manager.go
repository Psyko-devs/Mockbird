package cache

import (
	"fmt"
	"sort"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

type Store interface {
	Save(key string, entry Entry) error
	Delete(key string) error
	Clear() error
	Snapshot() ([]SnapshotEntry, error)
	Restore() ([]StoredEntry, error)
}

type StoredEntry struct {
	Key   string
	Entry Entry
}

type Options struct {
	MaxEntries int
	TTL        time.Duration
	Store      Store
}

type Manager struct {
	ttl              time.Duration
	ram              *lru.Cache[string, Entry]
	store            Store
	mu               sync.Mutex
	suppressEvict    bool
	pendingEvictions []string
}

type SnapshotEntry struct {
	Key        string     `json:"key"`
	StatusCode int        `json:"status_code"`
	Headers    HTTPHeader `json:"headers,omitempty"`
	BodySize   int        `json:"body_size"`
	CreatedAt  time.Time  `json:"created_at"`
	Source     string     `json:"source"`
	Expired    bool       `json:"expired"`
}

type HTTPHeader map[string][]string

func NewManager(opts Options) (*Manager, error) {
	if opts.MaxEntries <= 0 {
		return nil, fmt.Errorf("max entries must be positive")
	}
	if opts.TTL <= 0 {
		return nil, fmt.Errorf("ttl must be positive")
	}
	if opts.Store == nil {
		return nil, fmt.Errorf("store is required")
	}

	manager := &Manager{
		ttl:   opts.TTL,
		store: opts.Store,
	}

	ram, err := lru.NewWithEvict[string, Entry](opts.MaxEntries, func(key string, _ Entry) {
		if manager.suppressEvict {
			return
		}
		manager.pendingEvictions = append(manager.pendingEvictions, key)
	})
	if err != nil {
		return nil, err
	}
	manager.ram = ram

	if err := manager.restore(); err != nil {
		return nil, err
	}

	return manager, nil
}

func (m *Manager) Get(key string) (Entry, bool) {
	now := time.Now()

	m.mu.Lock()
	if entry, ok := m.ram.Get(key); ok {
		if entry.Expired(now, m.ttl) {
			m.suppressEvict = true
			m.ram.Remove(key)
			m.suppressEvict = false
			m.mu.Unlock()
			_ = m.store.Delete(key)
			return Entry{}, false
		}
		m.mu.Unlock()
		return entry, true
	}
	m.mu.Unlock()
	return Entry{}, false
}

func (m *Manager) Set(key string, entry Entry) error {
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	if err := m.store.Save(key, entry); err != nil {
		return err
	}

	m.mu.Lock()
	m.ram.Add(key, entry)
	evicted := m.takePendingEvictionsLocked()
	m.mu.Unlock()
	m.deleteEvicted(evicted)
	return nil
}

func (m *Manager) Delete(key string) error {
	if err := m.store.Delete(key); err != nil {
		return err
	}

	m.mu.Lock()
	m.suppressEvict = true
	m.ram.Remove(key)
	m.suppressEvict = false
	m.mu.Unlock()
	return nil
}

func (m *Manager) Clear() error {
	if err := m.store.Clear(); err != nil {
		return err
	}

	m.mu.Lock()
	m.suppressEvict = true
	m.ram.Purge()
	m.suppressEvict = false
	m.mu.Unlock()
	return nil
}

func (m *Manager) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ram.Len()
}

func (m *Manager) Snapshot() ([]SnapshotEntry, error) {
	now := time.Now()

	m.mu.Lock()
	seen := make(map[string]SnapshotEntry, m.ram.Len())
	for _, key := range m.ram.Keys() {
		entry, ok := m.ram.Peek(key)
		if !ok {
			continue
		}
		seen[key] = snapshot(key, entry, "ram", now, m.ttl)
	}
	m.mu.Unlock()

	diskEntries, err := m.store.Snapshot()
	if err != nil {
		return nil, err
	}
	for _, entry := range diskEntries {
		if _, ok := seen[entry.Key]; ok {
			continue
		}
		entry.Source = "disk"
		entry.Expired = now.Sub(entry.CreatedAt) > m.ttl
		seen[entry.Key] = entry
	}

	out := make([]SnapshotEntry, 0, len(seen))
	for _, entry := range seen {
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func snapshot(key string, entry Entry, source string, now time.Time, ttl time.Duration) SnapshotEntry {
	return SnapshotEntry{
		Key:        key,
		StatusCode: entry.StatusCode,
		Headers:    HTTPHeader(entry.Headers.Clone()),
		BodySize:   len(entry.Body),
		CreatedAt:  entry.CreatedAt,
		Source:     source,
		Expired:    entry.Expired(now, ttl),
	}
}

func (m *Manager) restore() error {
	entries, err := m.store.Restore()
	if err != nil {
		return err
	}

	now := time.Now()
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Entry.CreatedAt.Before(entries[j].Entry.CreatedAt)
	})

	m.mu.Lock()
	defer m.mu.Unlock()
	for _, restored := range entries {
		if restored.Entry.Expired(now, m.ttl) {
			_ = m.store.Delete(restored.Key)
			continue
		}
		m.ram.Add(restored.Key, restored.Entry)
		evicted := m.takePendingEvictionsLocked()
		m.mu.Unlock()
		m.deleteEvicted(evicted)
		m.mu.Lock()
	}
	return nil
}

func (m *Manager) takePendingEvictionsLocked() []string {
	if len(m.pendingEvictions) == 0 {
		return nil
	}
	evicted := append([]string(nil), m.pendingEvictions...)
	m.pendingEvictions = m.pendingEvictions[:0]
	return evicted
}

func (m *Manager) deleteEvicted(keys []string) {
	for _, key := range keys {
		_ = m.store.Delete(key)
	}
}
