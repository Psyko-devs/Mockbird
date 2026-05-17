package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Psyko-devs/mockbird/internal/cache"
)

type CacheManager interface {
	Snapshot() ([]cache.SnapshotEntry, error)
	Clear() error
	Delete(key string) error
}

func RegisterCacheRoutes(mux *http.ServeMux, manager CacheManager, logger *slog.Logger) {
	mux.HandleFunc("/__mockbird/cache", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			entries, err := manager.Snapshot()
			if err != nil {
				logger.Error("failed to inspect cache", "error", err)
				writeError(w, http.StatusInternalServerError, "failed to inspect cache")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
		case http.MethodDelete:
			if err := manager.Clear(); err != nil {
				logger.Error("failed to clear cache", "error", err)
				writeError(w, http.StatusInternalServerError, "failed to clear cache")
				return
			}
			logger.Info("cache cleared")
			w.WriteHeader(http.StatusNoContent)
		default:
			w.Header().Set("Allow", "GET, DELETE")
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/__mockbird/cache/", func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/__mockbird/cache/")
		if key == "" {
			writeError(w, http.StatusBadRequest, "cache key is required")
			return
		}
		if r.Method != http.MethodDelete {
			w.Header().Set("Allow", "DELETE")
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if err := manager.Delete(key); err != nil {
			logger.Warn("failed to delete cache entry", "key", key, "error", err)
			writeError(w, http.StatusBadRequest, "failed to delete cache entry")
			return
		}
		logger.Info("cache entry invalidated", "key", key)
		w.WriteHeader(http.StatusNoContent)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
