package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Psyko-devs/mockbird/internal/cache"
)

var ErrNotFound = errors.New("cache entry not found")

type DiskStore struct {
	dir string
	mu  sync.RWMutex
}

func NewDiskStore(dir string) (*DiskStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &DiskStore{dir: dir}, nil
}

func (s *DiskStore) Load(key string) (cache.Entry, error) {
	if err := validateKey(key); err != nil {
		return cache.Entry{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.path(key))
	if errors.Is(err, os.ErrNotExist) {
		return cache.Entry{}, ErrNotFound
	}
	if err != nil {
		return cache.Entry{}, err
	}

	var entry cache.Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		return cache.Entry{}, err
	}
	return entry, nil
}

func (s *DiskStore) Save(key string, entry cache.Entry) error {
	if err := validateKey(key); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, key+"-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.path(key))
}

func (s *DiskStore) Delete(key string) error {
	if err := validateKey(key); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	err := os.Remove(s.path(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
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
	return nil
}

func (s *DiskStore) ListKeys() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		key := strings.TrimSuffix(entry.Name(), ".json")
		if validateKey(key) == nil {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

func (s *DiskStore) path(key string) string {
	return filepath.Join(s.dir, key+".json")
}

func validateKey(key string) error {
	if len(key) != 64 {
		return fmt.Errorf("invalid cache key length")
	}
	for _, r := range key {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			continue
		}
		return fmt.Errorf("invalid cache key")
	}
	return nil
}
