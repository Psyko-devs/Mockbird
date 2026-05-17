package config

import (
	"flag"
	"fmt"
	"time"
)

type Config struct {
	Target   string
	Port     int
	Dir      string
	TTL      time.Duration
	MaxRAM   int
	Pretty   bool
	LogLevel string
}

func Parse(args []string) (Config, error) {
	cfg := Config{}
	fs := flag.NewFlagSet("mockbird", flag.ContinueOnError)
	fs.StringVar(&cfg.Target, "target", "https://jsonplaceholder.typicode.com", "upstream API to proxy and record")
	fs.IntVar(&cfg.Port, "port", 8080, "local port to listen on")
	fs.StringVar(&cfg.Dir, "dir", "./mockbird_cache", "directory for disk cache files")
	fs.DurationVar(&cfg.TTL, "ttl", 24*time.Hour, "cache entry time-to-live")
	fs.IntVar(&cfg.MaxRAM, "max-ram", 1000, "maximum L1 RAM cache entries")
	fs.BoolVar(&cfg.Pretty, "pretty", false, "pretty-print JSON responses before caching and replaying")
	fs.StringVar(&cfg.LogLevel, "log-level", "info", "log level: debug, info, warn, error")

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
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return Config{}, fmt.Errorf("log-level must be one of debug, info, warn, error")
	}

	return cfg, nil
}
