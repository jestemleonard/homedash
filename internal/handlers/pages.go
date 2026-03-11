package handlers

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/jestemleonard/homedash/internal/config"
	"github.com/jestemleonard/homedash/internal/engine"
	"github.com/jestemleonard/homedash/internal/renderer"
	"github.com/jestemleonard/homedash/internal/services"
)

// PageHandler handles page requests
type PageHandler struct {
	config         *config.Config
	renderer       *renderer.Renderer
	systemService  *services.SystemService
	weatherService *services.WeatherService
	engine         *engine.Engine
	hasCustomCSS   bool
}

// NewPageHandler creates a new page handler
func NewPageHandler(cfg *config.Config, r *renderer.Renderer, sys *services.SystemService, weather *services.WeatherService, eng *engine.Engine) *PageHandler {
	// Check for custom.css in override locations
	hasCustomCSS := false
	for _, path := range []string{"overrides/static/css/custom.css", "custom/static/css/custom.css"} {
		if _, err := os.Stat(path); err == nil {
			hasCustomCSS = true
			break
		}
	}

	return &PageHandler{
		config:         cfg,
		renderer:       r,
		systemService:  sys,
		weatherService: weather,
		engine:         eng,
		hasCustomCSS:   hasCustomCSS,
	}
}

// PageData contains all data for a page render
type PageData struct {
	Title           string
	Theme           *config.ThemeConfig
	HomePageEnabled bool
	Search          *config.SearchConfig
	Bookmarks       *config.BookmarksConfig
	Weather         any
	System          any
	Services        any
	CustomCSS       bool
}

// HandleIndex renders the main dashboard page
func (h *PageHandler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	// Get system stats
	system, err := h.systemService.GetStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get weather data (only when the home page is shown)
	var weather any
	if h.config.HomePage.Enabled && h.weatherService.IsEnabled() {
		weather, err = h.weatherService.GetWeather()
		if err != nil {
			slog.Warn("weather fetch failed", "error", err)
			weather = nil
		}
	}

	var services any
	if h.engine != nil {
		services = h.engine.ServiceGroups()
	}

	data := PageData{
		Title:           "Dashboard",
		Theme:           &h.config.Theme,
		HomePageEnabled: h.config.HomePage.Enabled,
		Search:          &h.config.Search,
		Bookmarks:       &h.config.Bookmarks,
		Weather:         weather,
		System:          system,
		Services:        services,
		CustomCSS:       h.hasCustomCSS,
	}

	if err := h.renderer.RenderPage(w, "index", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
