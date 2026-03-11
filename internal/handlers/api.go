package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jestemleonard/homedash/internal/engine"
	"github.com/jestemleonard/homedash/internal/models"
	"github.com/jestemleonard/homedash/internal/renderer"
	"github.com/jestemleonard/homedash/internal/services"
)

// APIHandler handles API endpoints
type APIHandler struct {
	systemService  *services.SystemService
	weatherService *services.WeatherService
	renderer       *renderer.Renderer
	engine         *engine.Engine
}

// NewAPIHandler creates a new API handler
func NewAPIHandler(sys *services.SystemService, weather *services.WeatherService, r *renderer.Renderer, eng *engine.Engine) *APIHandler {
	return &APIHandler{
		systemService:  sys,
		weatherService: weather,
		renderer:       r,
		engine:         eng,
	}
}

// HandleSystemStats returns system stats as JSON
func (h *APIHandler) HandleSystemStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.systemService.GetStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		slog.Error("failed to encode system stats", "error", err)
	}
}

// HandleSystemStatsHTML returns system stats as HTML (for HTMX)
func (h *APIHandler) HandleSystemStatsHTML(w http.ResponseWriter, r *http.Request) {
	stats, err := h.systemService.GetStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := h.renderer.RenderComponent(w, "stats", stats); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// HandleWeather returns weather data as JSON
func (h *APIHandler) HandleWeather(w http.ResponseWriter, r *http.Request) {
	weather, err := h.weatherService.GetWeather()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(weather); err != nil {
		slog.Error("failed to encode weather data", "error", err)
	}
}

// HandleWeatherHTML returns weather as HTML (for HTMX)
func (h *APIHandler) HandleWeatherHTML(w http.ResponseWriter, r *http.Request) {
	weather, err := h.weatherService.GetWeather()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := h.renderer.RenderComponent(w, "weather", weather); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// HandleServicesHTML returns service groups as HTML (for HTMX)
func (h *APIHandler) HandleServicesHTML(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.renderServicesHTML(w)
}

// renderServicesHTML renders all service groups as HTML fragments.
func (h *APIHandler) renderServicesHTML(w http.ResponseWriter) {
	if h.engine == nil {
		return
	}

	groups := h.engine.ServiceGroups()
	for _, g := range groups {
		if err := h.renderer.RenderComponentToWriter(w, "service-group", g); err != nil {
			slog.Error("failed to render service group", "id", g.ID, "error", err)
			return
		}
	}
}

// HandleWidgets returns widgets as JSON
func (h *APIHandler) HandleWidgets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if h.engine == nil {
		if err := json.NewEncoder(w).Encode([]any{}); err != nil {
			slog.Error("failed to encode widgets", "error", err)
		}
		return
	}

	widgets := h.engine.Widgets()
	if widgets == nil {
		widgets = []models.Widget{}
	}
	if err := json.NewEncoder(w).Encode(widgets); err != nil {
		slog.Error("failed to encode widgets", "error", err)
	}
}

// HandleAction triggers an integration action
func (h *APIHandler) HandleAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse path: /api/actions/{integration}/{action}
	path := strings.TrimPrefix(r.URL.Path, "/api/actions/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		http.Error(w, "invalid action path", http.StatusBadRequest)
		return
	}
	integrationID, actionID := parts[0], parts[1]

	if h.engine == nil {
		http.Error(w, "integration engine not available", http.StatusServiceUnavailable)
		return
	}

	action, auth, creds, serviceURL, ok := h.engine.FindAction(integrationID, actionID)
	if !ok {
		http.Error(w, "action not found", http.StatusNotFound)
		return
	}

	var bodyBytes []byte
	if action.Body != nil {
		var err error
		bodyBytes, err = json.Marshal(action.Body)
		if err != nil {
			http.Error(w, "failed to marshal action body", http.StatusInternalServerError)
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result, err := h.engine.Fetcher().FetchAction(ctx, serviceURL, action, auth, creds, bodyBytes)
	if err != nil {
		slog.Error("action failed", "integration", integrationID, "action", actionID, "error", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(result)
}

// HandleContainerToggle starts or stops a Docker container.
// Routes: POST /api/containers/{id}/start, POST /api/containers/{id}/stop
func (h *APIHandler) HandleContainerToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse path: /api/containers/{id}/{action}
	path := strings.TrimPrefix(r.URL.Path, "/api/containers/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		http.Error(w, "invalid path, expected /api/containers/{id}/{start|stop}", http.StatusBadRequest)
		return
	}
	containerID, action := parts[0], parts[1]

	if action != "start" && action != "stop" {
		http.Error(w, "action must be start or stop", http.StatusBadRequest)
		return
	}

	if h.engine == nil {
		http.Error(w, "integration engine not available", http.StatusServiceUnavailable)
		return
	}

	if !h.engine.HasContainer(containerID) {
		http.Error(w, "container not managed by homedash", http.StatusForbidden)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var err error
	if action == "start" {
		err = h.engine.ContainerStart(ctx, containerID)
	} else {
		err = h.engine.ContainerStop(ctx, containerID)
	}

	if err != nil {
		slog.Error("container action failed", "action", action, "container", containerID, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	slog.Info("container action succeeded", "action", action, "container", containerID)

	// Return the updated services HTML so HTMX swaps the services list immediately
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.renderServicesHTML(w)
}
