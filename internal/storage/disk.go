package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Psyko-devs/mockbird/internal/cache"
)

var (
	ErrNotFound   = errors.New("cache entry not found")
	ErrCorrupt    = errors.New("cache entry corrupt")
	errBadKey     = errors.New("invalid cache key")
	indexName     = "index.json"
	fileMode      = os.FileMode(0644)
	directoryMode = os.FileMode(0755)
	timeFormat    = time.RFC3339Nano
)

type diskRecord struct {
	StatusCode int         `json:"status_code"`
	Headers    http.Header `json:"headers"`
	Body       []byte      `json:"body"`
	CreatedAt  string      `json:"created_at"`
	Checksum   string      `json:"checksum"`
}

type indexRecord struct {
	Key        string      `json:"key"`
	StatusCode int         `json:"status_code"`
	Headers    http.Header `json:"headers,omitempty"`
	BodySize   int         `json:"body_size"`
	CreatedAt  string      `json:"created_at"`
	Checksum   string      `json:"checksum"`
}

type DiskStore struct {
	dir   string
	index map[string]cache.SnapshotEntry
	mu    sync.RWMutex
}

func NewDiskStore(dir string) (*DiskStore, error) {
	if err := os.MkdirAll(dir, directoryMode); err != nil {
		return nil, err
	}
	store := &DiskStore{dir: dir, index: make(map[string]cache.SnapshotEntry)}
	if err := store.loadIndex(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *DiskStore) Load(key string) (cache.Entry, error) {
	if err := validateKey(key); err != nil {
		return cache.Entry{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path(key))
	if errors.Is(err, os.ErrNotExist) {
		delete(s.index, key)
		_ = s.persistIndexLocked()
		return cache.Entry{}, ErrNotFound
	}
	if err != nil {
		return cache.Entry{}, err
	}

	var record diskRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return cache.Entry{}, fmt.Errorf("%w: %s", ErrCorrupt, key)
	}
	entry, err := record.entry()
	if err != nil {
		return cache.Entry{}, fmt.Errorf("%w: %s", ErrCorrupt, key)
	}
	if checksum(entry) != record.Checksum {
		return cache.Entry{}, fmt.Errorf("%w: checksum mismatch for %s", ErrCorrupt, key)
	}
	return entry, nil
}

func (s *DiskStore) Save(key string, entry cache.Entry) error {
	if err := validateKey(key); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record := newDiskRecord(entry)
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if err := s.atomicWrite(s.path(key), data); err != nil {
		return err
	}

	s.index[key] = snapshotFromEntry(key, entry, record.Checksum)
	return s.persistIndexLocked()
}

func (s *DiskStore) Delete(key string) error {
	if err := validateKey(key); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	err := os.Remove(s.path(key))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	delete(s.index, key)
	return s.persistIndexLocked()
}

func (s *DiskStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		if err := os.Remove(filepath.Join(s.dir, entry.Name())); err != nil {
			return err
		}
	}
	s.index = make(map[string]cache.SnapshotEntry)
	return s.persistIndexLocked()
}

func (s *DiskStore) Snapshot() ([]cache.SnapshotEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]cache.SnapshotEntry, 0, len(s.index))
	for _, entry := range s.index {
		out = append(out, entry)
	}
	return out, nil
}

func (s *DiskStore) loadIndex() error {
	data, err := os.ReadFile(s.indexPath())
	if errors.Is(err, os.ErrNotExist) {
		return s.rebuildIndex()
	}
	if err != nil {
		return err
	}

	var records []indexRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return s.rebuildIndex()
	}
	for _, record := range records {
		if validateKey(record.Key) != nil {
			continue
		}
		createdAt, err := parseTime(record.CreatedAt)
		if err != nil {
			continue
		}
		s.index[record.Key] = cache.SnapshotEntry{
			Key:        record.Key,
			StatusCode: record.StatusCode,
			Headers:    cache.HTTPHeader(record.Headers.Clone()),
			BodySize:   record.BodySize,
			CreatedAt:  createdAt,
			Source:     "disk",
		}
	}
	return nil
}

func (s *DiskStore) rebuildIndex() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" || entry.Name() == indexName {
			continue
		}
		key := strings.TrimSuffix(entry.Name(), ".json")
		if validateKey(key) != nil {
			continue
		}
		loaded, err := s.loadWithoutLock(key)
		if err != nil {
			continue
		}
		s.index[key] = snapshotFromEntry(key, loaded, checksum(loaded))
	}
	return s.persistIndexLocked()
}

func (s *DiskStore) loadWithoutLock(key string) (cache.Entry, error) {
	data, err := os.ReadFile(s.path(key))
	if err != nil {
		return cache.Entry{}, err
	}
	var record diskRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return cache.Entry{}, err
	}
	entry, err := record.entry()
	if err != nil {
		return cache.Entry{}, err
	}
	if checksum(entry) != record.Checksum {
		return cache.Entry{}, ErrCorrupt
	}
	return entry, nil
}

func (s *DiskStore) persistIndexLocked() error {
	records := make([]indexRecord, 0, len(s.index))
	for _, entry := range s.index {
		records = append(records, indexRecord{
			Key:        entry.Key,
			StatusCode: entry.StatusCode,
			Headers:    http.Header(entry.Headers).Clone(),
			BodySize:   entry.BodySize,
			CreatedAt:  entry.CreatedAt.Format(timeFormat),
		})
	}
	data, err := json.Marshal(records)
	if err != nil {
		return err
	}
	return s.atomicWrite(s.indexPath(), data)
}

func (s *DiskStore) atomicWrite(path string, data []byte) error {
	tmp, err := os.CreateTemp(s.dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, fileMode); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	return syncDir(s.dir)
}

func (s *DiskStore) path(key string) string {
	return filepath.Join(s.dir, key+".json")
}

func (s *DiskStore) indexPath() string {
	return filepath.Join(s.dir, indexName)
}

func validateKey(key string) error {
	if len(key) != 64 {
		return fmt.Errorf("%w: length", errBadKey)
	}
	for _, r := range key {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			continue
		}
		return fmt.Errorf("%w: characters", errBadKey)
	}
	return nil
}

func checksum(entry cache.Entry) string {
	sum := sha256.New()
	sum.Write([]byte(fmt.Sprintf("%d\n", entry.StatusCode)))
	sum.Write([]byte(entry.CreatedAt.UTC().Format(timeFormat)))
	sum.Write([]byte{'\n'})
	headerBytes, _ := json.Marshal(entry.Headers)
	sum.Write(headerBytes)
	sum.Write([]byte{'\n'})
	sum.Write(entry.Body)
	return hex.EncodeToString(sum.Sum(nil))
}

func newDiskRecord(entry cache.Entry) diskRecord {
	return diskRecord{
		StatusCode: entry.StatusCode,
		Headers:    entry.Headers,
		Body:       entry.Body,
		CreatedAt:  entry.CreatedAt.UTC().Format(timeFormat),
		Checksum:   checksum(entry),
	}
}

func (r diskRecord) entry() (cache.Entry, error) {
	createdAt, err := parseTime(r.CreatedAt)
	if err != nil {
		return cache.Entry{}, err
	}
	return cache.Entry{
		StatusCode: r.StatusCode,
		Headers:    r.Headers,
		Body:       r.Body,
		CreatedAt:  createdAt,
	}, nil
}

func snapshotFromEntry(key string, entry cache.Entry, _ string) cache.SnapshotEntry {
	return cache.SnapshotEntry{
		Key:        key,
		StatusCode: entry.StatusCode,
		Headers:    cache.HTTPHeader(entry.Headers.Clone()),
		BodySize:   entry.BodySize(),
		CreatedAt:  entry.CreatedAt,
		Source:     "disk",
	}
}

func parseTime(value string) (time.Time, error) {
	t, err := time.Parse(timeFormat, value)
	if err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, value)
}

func syncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := f.Sync(); err != nil && runtime.GOOS != "windows" {
		return err
	}
	return nil
}
