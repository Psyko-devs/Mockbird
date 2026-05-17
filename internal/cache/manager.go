package cache

import (
	"fmt"
	"sort"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

type Store interface {
	Load(key string) (Entry, error)
	Save(key string, entry Entry) error
	Delete(key string) error
	Clear() error
	Snapshot() ([]SnapshotEntry, error)
}

type Options struct {
	MaxEntries int
	TTL        time.Duration
	Store      Store
}

type Manager struct {
	ttl   time.Duration
	ram   *lru.Cache[string, Entry]
	store Store
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

	ram, err := lru.New[string, Entry](opts.MaxEntries)
	if err != nil {
		return nil, err
	}

	return &Manager{ttl: opts.TTL, ram: ram, store: opts.Store}, nil
}

func (m *Manager) Get(key string) (Entry, bool) {
	now := time.Now()
	if entry, ok := m.ram.Get(key); ok {
		if entry.Expired(now, m.ttl) {
			m.ram.Remove(key)
			_ = m.store.Delete(key)
			return Entry{}, false
		}
		return entry, true
	}

	entry, err := m.store.Load(key)
	if err != nil {
		return Entry{}, false
	}
	if entry.Expired(now, m.ttl) {
		_ = m.store.Delete(key)
		return Entry{}, false
	}

	m.ram.Add(key, entry)
	return entry, true
}

func (m *Manager) Set(key string, entry Entry) error {
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	if err := m.store.Save(key, entry); err != nil {
		return err
	}
	m.ram.Add(key, entry)
	return nil
}

func (m *Manager) Delete(key string) error {
	m.ram.Remove(key)
	return m.store.Delete(key)
}

func (m *Manager) Clear() error {
	m.ram.Purge()
	return m.store.Clear()
}

func (m *Manager) Len() int {
	return m.ram.Len()
}

func (m *Manager) Snapshot() ([]SnapshotEntry, error) {
	seen := make(map[string]SnapshotEntry, m.ram.Len())
	now := time.Now()

	for _, key := range m.ram.Keys() {
		entry, ok := m.ram.Peek(key)
		if !ok {
			continue
		}
		seen[key] = snapshot(key, entry, "ram", now, m.ttl)
	}

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
