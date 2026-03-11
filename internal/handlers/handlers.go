package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jestemleonard/homedash/internal/config"
	"github.com/jestemleonard/homedash/internal/engine"
	"github.com/jestemleonard/homedash/internal/renderer"
	"github.com/jestemleonard/homedash/internal/services"
)

// Handlers contains all HTTP handlers
type Handlers struct {
	Pages  *PageHandler
	API    *APIHandler
	Static *StaticHandler
}

// New creates all handlers
func New(cfg *config.Config, r *renderer.Renderer) (*Handlers, error) {
	// Create services
	systemService := services.NewSystemService()
	weatherService := services.NewWeatherService(&cfg.Weather)

	// Create integration engine
	eng, err := engine.New("integrations", cfg.Services, cfg.Exclude, cfg.ServiceOrder, cfg.Server.Hostname)
	if err != nil {
		slog.Error("failed to create integration engine", "error", err)
		// Non-fatal: continue without integrations
		eng = nil
	}

	if eng != nil {
		eng.Start(context.Background())
	}

	return &Handlers{
		Pages:  NewPageHandler(cfg, r, systemService, weatherService, eng),
		API:    NewAPIHandler(systemService, weatherService, r, eng),
		Static: NewStaticHandler(cfg.Server.StaticDir),
	}, nil
}

// SetupRoutes configures all HTTP routes
func (h *Handlers) SetupRoutes(mux *http.ServeMux) {
	// Pages
	mux.HandleFunc("/", h.Pages.HandleIndex)

	// API endpoints - JSON
	mux.HandleFunc("/api/system", h.API.HandleSystemStats)
	mux.HandleFunc("/api/weather", h.API.HandleWeather)
	mux.HandleFunc("/api/widgets", h.API.HandleWidgets)
	mux.HandleFunc("/api/actions/", h.API.HandleAction)
	mux.HandleFunc("/api/containers/", h.API.HandleContainerToggle)

	// API endpoints - HTML (for HTMX)
	mux.HandleFunc("/api/system/html", h.API.HandleSystemStatsHTML)
	mux.HandleFunc("/api/weather/html", h.API.HandleWeatherHTML)
	mux.HandleFunc("/api/services/html", h.API.HandleServicesHTML)

	// Static files
	mux.Handle("/static/", h.Static)
}

// LoggingMiddleware logs HTTP requests with method, path, and duration
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
	})
}
