package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/jestemleonard/homedash/internal/config"
	"github.com/jestemleonard/homedash/internal/handlers"
	"github.com/jestemleonard/homedash/internal/renderer"
)

func main() {
	// Set up structured logging
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	// Load configuration
	configPath := "config.yaml"
	if envPath := os.Getenv("HOMEDASH_CONFIG"); envPath != "" {
		configPath = envPath
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Warn("could not load config, using defaults", "path", configPath, "error", err)
		cfg = config.DefaultConfig()
	}

	// Initialize renderer
	r, err := renderer.New(cfg.Server.TemplatesDir)
	if err != nil {
		slog.Error("failed to initialize renderer", "error", err)
		os.Exit(1)
	}

	// Initialize handlers
	h, err := handlers.New(cfg, r)
	if err != nil {
		slog.Error("failed to initialize handlers", "error", err)
		os.Exit(1)
	}

	// Setup routes
	mux := http.NewServeMux()
	h.SetupRoutes(mux)

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	slog.Info("starting homedash server", "addr", addr, "url", fmt.Sprintf("http://localhost:%d/", cfg.Server.Port))

	if err := http.ListenAndServe(addr, handlers.LoggingMiddleware(mux)); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
