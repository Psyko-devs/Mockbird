package config

import (
	"flag"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Config struct {
	Target      string
	Port        int
	Dir         string
	TTL         time.Duration
	MaxRAM      int
	Pretty      bool
	LogLevel    string
	VaryHeaders []string
	MaxBody     int64
	MaxResponse int64
}

func Parse(args []string) (Config, error) {
	cfg := Config{}
	fs := flag.NewFlagSet("mockbird", flag.ContinueOnError)
	fs.StringVar(&cfg.Target, "target", "https://jsonplaceholder.typicode.com", "upstream API to proxy and record")
	fs.IntVar(&cfg.Port, "port", 8080, "local port to listen on")
	fs.StringVar(&cfg.Dir, "dir", "./mockbird_cache", "directory for disk cache files")
	fs.DurationVar(&cfg.TTL, "ttl", 24*time.Hour, "cache entry time-to-live")
	fs.IntVar(&cfg.MaxRAM, "max-ram", 1000, "maximum L1 RAM cache entries")
	fs.BoolVar(&cfg.Pretty, "pretty", false, "pretty-print JSON responses at replay time without changing stored cache bytes")
	fs.StringVar(&cfg.LogLevel, "log-level", "info", "log level: debug, info, warn, error")
	vary := fs.String("vary", "", "comma-separated request headers that participate in cache keys, for example Authorization,Accept")
	fs.Int64Var(&cfg.MaxBody, "max-body", 10<<20, "maximum request body bytes to read for cache keys")
	fs.Int64Var(&cfg.MaxResponse, "max-response", 100<<20, "maximum upstream response body bytes to record")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return Config{}, fmt.Errorf("port must be between 1 and 65535")
	}
	if cfg.TTL <= 0 {
		return Config{}, fmt.Errorf("ttl must be positive")
	}
	if cfg.MaxRAM <= 0 {
		return Config{}, fmt.Errorf("max-ram must be positive")
	}
	if cfg.MaxBody <= 0 {
		return Config{}, fmt.Errorf("max-body must be positive")
	}
	if cfg.MaxResponse <= 0 {
		return Config{}, fmt.Errorf("max-response must be positive")
	}
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return Config{}, fmt.Errorf("log-level must be one of debug, info, warn, error")
	}
	cfg.VaryHeaders = parseHeaderList(*vary)

	return cfg, nil
}

func parseHeaderList(raw string) []string {
	if raw == "" {
		return nil
	}
	seen := make(map[string]struct{})
	headers := make([]string, 0)
	for _, part := range strings.Split(raw, ",") {
		header := http.CanonicalHeaderKey(strings.TrimSpace(part))
		if header == "" {
			continue
		}
		if _, ok := seen[header]; ok {
			continue
		}
		seen[header] = struct{}{}
		headers = append(headers, header)
	}
	return headers
}
