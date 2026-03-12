package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Override directories checked before default static dir
var staticOverrideDirs = []string{"overrides/static"}

// StaticHandler serves static files with override support
type StaticHandler struct {
	staticDir    string
	overrideDirs []string
}

// NewStaticHandler creates a new static file handler
func NewStaticHandler(staticDir string) *StaticHandler {
	return &StaticHandler{
		staticDir:    staticDir,
		overrideDirs: staticOverrideDirs,
	}
}

// ServeHTTP handles static file requests
func (h *StaticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Strip /static/ prefix
	path := strings.TrimPrefix(r.URL.Path, "/static/")

	// Security: prevent directory traversal
	if strings.Contains(path, "..") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Set appropriate content type
	ext := filepath.Ext(path)
	switch ext {
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".woff2":
		w.Header().Set("Content-Type", "font/woff2")
	case ".woff":
		w.Header().Set("Content-Type", "font/woff")
	}

	// Set caching headers for production
	w.Header().Set("Cache-Control", "public, max-age=3600")

	// Check override directories first
	for _, dir := range h.overrideDirs {
		candidate := filepath.Join(dir, path)
		if _, err := os.Stat(candidate); err == nil {
			http.ServeFile(w, r, candidate)
			return
		}
	}

	// Fall back to default static directory
	filePath := filepath.Join(h.staticDir, path)
	http.ServeFile(w, r, filePath)
}
