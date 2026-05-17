package storage

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/Psyko-devs/mockbird/internal/cache"
)

var (
	ErrNotFound   = errors.New("cache entry not found")
	ErrCorrupt    = errors.New("cache wal record corrupt")
	errBadKey     = errors.New("invalid cache key")
	directoryMode = os.FileMode(0755)
	fileMode      = os.FileMode(0644)
	walName       = "cache.wal"
)

type WALEntry struct {
	Op        string       `json:"op"`
	Key       string       `json:"key,omitempty"`
	Entry     *cache.Entry `json:"entry,omitempty"`
	Timestamp int64        `json:"timestamp"`
}

type DiskStore struct {
	dir             string
	walPath         string
	entries         map[string]cache.Entry
	mu              sync.RWMutex
	walMu           sync.Mutex
	compactionEvery time.Duration
}

func NewDiskStore(dir string) (*DiskStore, error) {
	if err := os.MkdirAll(dir, directoryMode); err != nil {
		return nil, err
	}

	store := &DiskStore{
		dir:             dir,
		walPath:         filepath.Join(dir, walName),
		entries:         make(map[string]cache.Entry),
		compactionEvery: time.Minute,
	}
	if err := store.replayWAL(); err != nil {
		return nil, err
	}
	go store.compactionLoop()
	return store, nil
}

func (s *DiskStore) Restore() ([]cache.StoredEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := make([]cache.StoredEntry, 0, len(s.entries))
	for key, entry := range s.entries {
		entries = append(entries, cache.StoredEntry{Key: key, Entry: entry})
	}
	return entries, nil
}

func (s *DiskStore) Save(key string, entry cache.Entry) error {
	if err := validateKey(key); err != nil {
		return err
	}

	record := WALEntry{
		Op:        "set",
		Key:       key,
		Entry:     &entry,
		Timestamp: time.Now().UnixNano(),
	}
	if err := s.appendWAL(record); err != nil {
		return err
	}

	s.mu.Lock()
	s.entries[key] = entry
	s.mu.Unlock()
	return nil
}

func (s *DiskStore) Delete(key string) error {
	if err := validateKey(key); err != nil {
		return err
	}

	record := WALEntry{
		Op:        "delete",
		Key:       key,
		Timestamp: time.Now().UnixNano(),
	}
	if err := s.appendWAL(record); err != nil {
		return err
	}

	s.mu.Lock()
	delete(s.entries, key)
	s.mu.Unlock()
	return nil
}

func (s *DiskStore) Clear() error {
	record := WALEntry{
		Op:        "clear",
		Timestamp: time.Now().UnixNano(),
	}
	if err := s.appendWAL(record); err != nil {
		return err
	}

	s.mu.Lock()
	s.entries = make(map[string]cache.Entry)
	s.mu.Unlock()
	return nil
}

func (s *DiskStore) Snapshot() ([]cache.SnapshotEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]cache.SnapshotEntry, 0, len(s.entries))
	for key, entry := range s.entries {
		out = append(out, cache.SnapshotEntry{
			Key:        key,
			StatusCode: entry.StatusCode,
			Headers:    cache.HTTPHeader(entry.Headers.Clone()),
			BodySize:   entry.BodySize(),
			CreatedAt:  entry.CreatedAt,
			Source:     "disk",
			Expired:    false,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func (s *DiskStore) appendWAL(record WALEntry) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	s.walMu.Lock()
	defer s.walMu.Unlock()

	file, err := os.OpenFile(s.walPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, fileMode)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func (s *DiskStore) replayWAL() error {
	file, err := os.Open(s.walPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 && err == nil {
			s.applyWALLine(line)
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func (s *DiskStore) applyWALLine(line []byte) {
	var record WALEntry
	if err := json.Unmarshal(line, &record); err != nil {
		return
	}

	switch record.Op {
	case "set":
		if record.Entry == nil || validateKey(record.Key) != nil {
			return
		}
		s.entries[record.Key] = *record.Entry
	case "delete":
		if validateKey(record.Key) != nil {
			return
		}
		delete(s.entries, record.Key)
	case "clear":
		s.entries = make(map[string]cache.Entry)
	}
}

func (s *DiskStore) compactionLoop() {
	ticker := time.NewTicker(s.compactionEvery)
	defer ticker.Stop()
	for range ticker.C {
		_ = s.Compact()
	}
}

func (s *DiskStore) Compact() error {
	s.walMu.Lock()
	defer s.walMu.Unlock()

	s.mu.RLock()
	records := make([]WALEntry, 0, len(s.entries))
	keys := make([]string, 0, len(s.entries))
	for key := range s.entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		entry := s.entries[key]
		records = append(records, WALEntry{
			Op:        "set",
			Key:       key,
			Entry:     &entry,
			Timestamp: time.Now().UnixNano(),
		})
	}
	s.mu.RUnlock()

	tmp, err := os.CreateTemp(s.dir, ".cache-wal-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	writer := bufio.NewWriter(tmp)
	for _, record := range records {
		data, err := json.Marshal(record)
		if err != nil {
			_ = tmp.Close()
			return err
		}
		if _, err := writer.Write(append(data, '\n')); err != nil {
			_ = tmp.Close()
			return err
		}
	}
	if err := writer.Flush(); err != nil {
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
	return os.Rename(tmpName, s.walPath)
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
