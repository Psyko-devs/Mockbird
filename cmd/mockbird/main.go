package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Psyko-devs/mockbird/internal/cache"
	"github.com/Psyko-devs/mockbird/internal/config"
	"github.com/Psyko-devs/mockbird/internal/proxy"
	"github.com/Psyko-devs/mockbird/internal/server"
	"github.com/Psyko-devs/mockbird/internal/storage"
)

func main() {
	cfg, err := config.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(2)
	}

	logger, err := newLogger(cfg.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(2)
	}

	target, err := url.Parse(cfg.Target)
	if err != nil {
		logger.Error("invalid target url", "target", cfg.Target, "error", err)
		os.Exit(2)
	}
	if target.Scheme == "" || target.Host == "" {
		logger.Error("target must include scheme and host", "target", cfg.Target)
		os.Exit(2)
	}

	disk, err := storage.NewDiskStore(cfg.Dir)
	if err != nil {
		logger.Error("failed to initialize disk cache", "dir", cfg.Dir, "error", err)
		os.Exit(1)
	}

	manager, err := cache.NewManager(cache.Options{
		MaxEntries: cfg.MaxRAM,
		TTL:        cfg.TTL,
		Store:      disk,
	})
	if err != nil {
		logger.Error("failed to initialize cache manager", "error", err)
		os.Exit(1)
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	proxyHandler := proxy.New(proxy.Options{
		Target: target,
		Cache:  manager,
		Client: &http.Client{
			Transport: transport,
			Timeout:   60 * time.Second,
		},
		Pretty:      cfg.Pretty,
		VaryHeaders: cfg.VaryHeaders,
		MaxBody:     cfg.MaxBody,
		MaxResponse: cfg.MaxResponse,
		Logger:      logger,
	})

	mux := http.NewServeMux()
	server.RegisterCacheRoutes(mux, manager, logger)
	mux.Handle("/", proxyHandler)

	httpServer := &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", cfg.Port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("mockbird started",
			"addr", "http://"+httpServer.Addr,
			"target", cfg.Target,
			"cache_dir", cfg.Dir,
			"ttl", cfg.TTL.String(),
			"max_ram", cfg.MaxRAM,
			"vary", cfg.VaryHeaders,
		)
		errCh <- httpServer.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-stop:
		logger.Info("shutdown signal received", "signal", sig.String())
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			logger.Error("server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("mockbird stopped")
}

func newLogger(level string) (*slog.Logger, error) {
	var slogLevel slog.Level
	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "info":
		slogLevel = slog.LevelInfo
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		return nil, fmt.Errorf("unsupported log level %q", level)
	}

	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel})), nil
}
