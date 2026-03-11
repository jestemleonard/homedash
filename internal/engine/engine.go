package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jestemleonard/homedash/internal/config"
	"github.com/jestemleonard/homedash/internal/discovery"
	"github.com/jestemleonard/homedash/internal/models"
)

// Engine orchestrates integration polling, caching, and widget building.
type Engine struct {
	integrations []IntegrationDef
	services     map[string]config.ServiceConfig
	exclude      map[string]bool
	fetcher      *Fetcher
	discovery    *discovery.Discovery
	states       map[string]*serviceState // keyed by integration ID
	order        []string                 // integration IDs in activation order
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
}

// serviceState holds runtime state for a single active integration.
type serviceState struct {
	integration IntegrationDef
	credentials ServiceCredentials
	url         string // internal URL used for API polling
	displayURL  string // user-facing URL for links (may differ from url)
	containerID string
	stopped     bool   // true when container is stopped by user toggle
	endpoints   map[string]*endpointCache
	mu          sync.RWMutex
}

// endpointCache holds extracted data for a single endpoint.
type endpointCache struct {
	data      map[string]any
	fetchedAt time.Time
	ttl       time.Duration
	err       error
}

// New creates an Engine, loads integration definitions, and matches them
// to configured services. The hostname parameter is used to rewrite
// discovered service URLs (e.g. Docker gateway IP → real host IP).
func New(integrationDir string, services map[string]config.ServiceConfig, exclude []string, serviceOrder []string, hostname string) (*Engine, error) {
	defs, err := LoadIntegrations(integrationDir)
	if err != nil {
		return nil, fmt.Errorf("loading integrations: %w", err)
	}

	// Auto-discover Docker services
	disc, discErr := discovery.New()
	if discErr != nil {
		slog.Warn("docker discovery init failed", "error", discErr)
	} else if disc != nil {
		// Convert IntegrationDefs to discovery.IntegrationInfo (avoids import cycle)
		infos := make([]discovery.IntegrationInfo, len(defs))
		for i, d := range defs {
			infos[i] = discovery.IntegrationInfo{
				ID:              d.ID,
				Images:          d.Detection.Images,
				DefaultPort:     d.Detection.DefaultPort,
				ConfigDiscovery: d.Detection.ConfigDiscovery,
			}
		}

		discovered, err := disc.Discover(infos, hostname)
		if err != nil {
			slog.Warn("docker discovery failed", "error", err)
		} else {
			if len(discovered) == 0 {
				slog.Info("docker discovery completed, no matching containers found")
			}
			// Merge discovered and manual config:
			// - URL provided manually → full override of discovery
			// - API key only, no URL → inject key into discovered entry (useful
			//   for services like Jellyfin where the key can't be auto-extracted)
			// - Both empty → don't touch the discovery result
			merged := make(map[string]config.ServiceConfig)
			for k, v := range discovered {
				merged[k] = v
				slog.Info("discovered service", "id", k, "url", v.URL)
			}
			for k, v := range services {
				if v.URL != "" {
					merged[k] = v
				} else if existing, ok := merged[k]; ok {
					// Inject any manually provided fields into the discovered entry.
					if v.APIKey != "" {
						existing.APIKey = v.APIKey
					}
					if v.ExternalURL != "" {
						existing.ExternalURL = v.ExternalURL
					}
					merged[k] = existing
				}
			}
			services = merged
		}
	}

	excludeMap := make(map[string]bool, len(exclude))
	for _, id := range exclude {
		excludeMap[id] = true
	}

	e := &Engine{
		integrations: defs,
		services:     services,
		exclude:      excludeMap,
		fetcher:      NewFetcher(),
		discovery:    disc,
		states:       make(map[string]*serviceState),
	}

	// Match integrations to configured services
	for _, def := range defs {
		if excludeMap[def.ID] {
			slog.Info("integration excluded", "id", def.ID)
			continue
		}

		svc, ok := services[def.ID]
		if !ok {
			slog.Info("no service config for integration, skipping", "id", def.ID)
			continue
		}

		if svc.URL == "" {
			slog.Warn("service config missing URL", "id", def.ID)
			continue
		}

		creds := ServiceCredentials{
			APIKey: svc.APIKey,
		}

		// Compute user-facing display URL:
		// 1. Per-service ExternalURL takes priority (user controls the full URL)
		// 2. Otherwise use the (possibly hostname-rewritten) URL
		dispURL := svc.URL
		if svc.ExternalURL != "" {
			dispURL = svc.ExternalURL
		}

		state := &serviceState{
			integration: def,
			credentials: creds,
			url:         svc.URL,
			displayURL:  dispURL,
			containerID: svc.ContainerID,
			endpoints:   make(map[string]*endpointCache),
		}

		for epName, ep := range def.API.Endpoints {
			state.endpoints[epName] = &endpointCache{
				ttl: ep.Interval.Duration,
			}
		}

		e.states[def.ID] = state
		e.order = append(e.order, def.ID)
		slog.Info("integration activated", "id", def.ID, "url", svc.URL)
	}

	// Apply user-defined service ordering if provided.
	if len(serviceOrder) > 0 {
		e.order = applyOrder(e.order, serviceOrder)
	}

	return e, nil
}

// applyOrder reorders ids so that items in priority come first (in priority order),
// followed by any remaining items in their original order.
func applyOrder(ids []string, priority []string) []string {
	active := make(map[string]bool, len(ids))
	for _, id := range ids {
		active[id] = true
	}

	placed := make(map[string]bool, len(priority))
	result := make([]string, 0, len(ids))

	for _, id := range priority {
		if active[id] && !placed[id] {
			result = append(result, id)
			placed[id] = true
		}
	}

	for _, id := range ids {
		if !placed[id] {
			result = append(result, id)
		}
	}

	return result
}

// Start launches polling goroutines for all active integrations.
func (e *Engine) Start(ctx context.Context) {
	e.ctx, e.cancel = context.WithCancel(ctx)

	for _, state := range e.states {
		for epName, ep := range state.integration.API.Endpoints {
			go e.pollEndpoint(e.ctx, state, epName, ep)
		}
	}

	slog.Info("integration engine started", "active", len(e.states))
}

// Stop cancels all polling goroutines and releases resources.
func (e *Engine) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
	if e.discovery != nil {
		e.discovery.Close()
	}
}

// ContainerStart starts a Docker container by ID, clears the stopped flag,
// and triggers an immediate re-poll of all endpoints.
func (e *Engine) ContainerStart(ctx context.Context, containerID string) error {
	if e.discovery == nil {
		return fmt.Errorf("docker not available")
	}
	if err := e.discovery.ContainerStart(ctx, containerID); err != nil {
		return err
	}

	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, st := range e.states {
		if st.containerID != containerID {
			continue
		}
		st.mu.Lock()
		st.stopped = false
		for _, cache := range st.endpoints {
			cache.err = nil
		}
		st.mu.Unlock()
		// Re-poll with retries in the background — gives the container
		// time to boot before we try to hit its API.
		go e.retryFetchAll(st)
		return nil
	}
	return nil
}

// ContainerStop sets the stopped flag immediately for instant UI feedback,
// then stops the Docker container in the background.
func (e *Engine) ContainerStop(ctx context.Context, containerID string) error {
	if e.discovery == nil {
		return fmt.Errorf("docker not available")
	}

	// Mark stopped immediately so the UI response is instant
	e.mu.RLock()
	for _, st := range e.states {
		if st.containerID != containerID {
			continue
		}
		st.mu.Lock()
		st.stopped = true
		st.mu.Unlock()
		break
	}
	e.mu.RUnlock()

	// Docker stop blocks for seconds (SIGTERM + graceful shutdown) — run in background
	go func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := e.discovery.ContainerStop(stopCtx, containerID); err != nil {
			slog.Error("background container stop failed", "container", containerID, "error", err)
			// Revert the stopped flag so UI reflects the real state
			e.mu.RLock()
			for _, st := range e.states {
				if st.containerID != containerID {
					continue
				}
				st.mu.Lock()
				st.stopped = false
				st.mu.Unlock()
				break
			}
			e.mu.RUnlock()
		}
	}()

	return nil
}

// HasContainer checks whether a container ID belongs to a managed service.
func (e *Engine) HasContainer(containerID string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, st := range e.states {
		if st.containerID == containerID {
			return true
		}
	}
	return false
}

// Widgets builds and returns all widgets from cached data.
func (e *Engine) Widgets() []models.Widget {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var widgets []models.Widget

	for _, id := range e.order {
		state := e.states[id]
		state.mu.RLock()

		if state.stopped {
			// Container stopped by user — show as off
			for _, wdef := range state.integration.Widgets {
				w := BuildWidget(state.integration, wdef, nil, state.displayURL, false)
				w.Health = models.HealthStatusOff
				widgets = append(widgets, w)
			}
			state.mu.RUnlock()
			continue
		}

		for _, wdef := range state.integration.Widgets {
			cache, ok := state.endpoints[wdef.Endpoint]
			if !ok {
				continue
			}

			healthy := cache.err == nil && cache.data != nil
			stale := !cache.fetchedAt.IsZero() && cache.ttl > 0 &&
				time.Since(cache.fetchedAt) > cache.ttl*2

			w := BuildWidget(state.integration, wdef, cache.data, state.displayURL, healthy)

			if cache.err != nil {
				w.Error = cache.err.Error()
				if cache.data != nil {
					// Have stale data — show warning instead of error
					w.Health = models.HealthStatusWarning
					w.Warning = "stale data: " + cache.err.Error()
					w.Error = ""
				}
			} else if stale {
				w.Health = models.HealthStatusWarning
				w.Warning = "data may be stale"
			}

			widgets = append(widgets, w)
		}

		state.mu.RUnlock()
	}

	return widgets
}

// ServiceGroups returns widgets grouped by their integration/service.
func (e *Engine) ServiceGroups() []models.ServiceGroup {
	widgets := e.Widgets()
	if len(widgets) == 0 {
		return nil
	}

	// Preserve insertion order
	var order []string
	groups := make(map[string]*models.ServiceGroup)

	for _, w := range widgets {
		g, ok := groups[w.Integration]
		if !ok {
			var containerID string
			e.mu.RLock()
			if st, exists := e.states[w.Integration]; exists {
				containerID = st.containerID
			}
			e.mu.RUnlock()

			g = &models.ServiceGroup{
				ID:          w.Integration,
				Name:        w.IntegrationName,
				Icon:        w.Icon,
				Color:       w.IntegrationColor,
				URL:         w.URL,
				Health:      models.HealthStatusHealthy,
				ContainerID: containerID,
			}
			groups[w.Integration] = g
			order = append(order, w.Integration)
		}

		g.Widgets = append(g.Widgets, w)

		// Group health is the worst of its widgets
		if w.Health == models.HealthStatusError {
			g.Health = models.HealthStatusError
		} else if w.Health == models.HealthStatusOff && g.Health == models.HealthStatusHealthy {
			g.Health = models.HealthStatusOff
		} else if w.Health == models.HealthStatusWarning && g.Health != models.HealthStatusError {
			g.Health = models.HealthStatusWarning
		}
	}

	result := make([]models.ServiceGroup, 0, len(order))
	for _, id := range order {
		result = append(result, *groups[id])
	}
	return result
}

// FindAction finds an action definition by integration and action ID.
func (e *Engine) FindAction(integrationID, actionID string) (ActionDef, AuthDef, ServiceCredentials, string, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	state, ok := e.states[integrationID]
	if !ok {
		return ActionDef{}, AuthDef{}, ServiceCredentials{}, "", false
	}

	for _, action := range state.integration.Actions {
		if action.ID == actionID {
			return action, state.integration.Auth, state.credentials, state.url, true
		}
	}
	return ActionDef{}, AuthDef{}, ServiceCredentials{}, "", false
}

// Fetcher returns the engine's HTTP fetcher for use by action handlers.
func (e *Engine) Fetcher() *Fetcher {
	return e.fetcher
}

func (e *Engine) pollEndpoint(ctx context.Context, state *serviceState, epName string, ep EndpointDef) {
	interval := ep.Interval.Duration
	if interval <= 0 {
		interval = 60 * time.Second
	}

	logger := slog.With("integration", state.integration.ID, "endpoint", epName)
	logger.Info("polling started", "interval", interval)

	// Fetch immediately on start
	e.fetchAndCache(ctx, state, epName, ep, logger)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("polling stopped")
			return
		case <-ticker.C:
			e.fetchAndCache(ctx, state, epName, ep, logger)
		}
	}
}

func (e *Engine) fetchAndCache(ctx context.Context, state *serviceState, epName string, ep EndpointDef, logger *slog.Logger) {
	// Skip fetching if container is stopped by user
	state.mu.RLock()
	stopped := state.stopped
	state.mu.RUnlock()
	if stopped {
		return
	}

	raw, err := e.fetcher.Fetch(ctx, state.url, state.integration.API.BasePath, ep, state.integration.Auth, state.credentials)
	if err != nil {
		logger.Error("fetch failed", "error", err)

		state.mu.Lock()
		cache := state.endpoints[epName]
		cache.err = err
		// Keep existing data (stale cache pattern)
		state.mu.Unlock()
		return
	}

	var data map[string]any
	if ep.Mapping != nil {
		data, err = Extract(raw, ep.Mapping)
		if err != nil {
			logger.Error("extraction failed", "error", err)

			state.mu.Lock()
			cache := state.endpoints[epName]
			cache.err = fmt.Errorf("extract: %w", err)
			state.mu.Unlock()
			return
		}
	} else {
		// No mapping — store raw JSON as generic map
		if jsonErr := json.Unmarshal(raw, &data); jsonErr != nil {
			// Try as array
			var arr []any
			if jsonErr2 := json.Unmarshal(raw, &arr); jsonErr2 == nil {
				data = map[string]any{"items": arr}
			}
		}
	}

	state.mu.Lock()
	cache := state.endpoints[epName]
	cache.data = data
	cache.fetchedAt = time.Now()
	cache.err = nil
	state.mu.Unlock()

	logger.Debug("fetch complete", "keys", mapKeys(data))
}

// retryFetchAll re-polls all endpoints for a service with retries,
// giving the container time to start before hitting its API.
func (e *Engine) retryFetchAll(st *serviceState) {
	for attempt := 1; attempt <= 3; attempt++ {
		time.Sleep(time.Duration(attempt) * 2 * time.Second) // 2s, 4s, 6s

		st.mu.RLock()
		if st.stopped {
			st.mu.RUnlock()
			return
		}
		st.mu.RUnlock()

		allOK := true
		for epName, ep := range st.integration.API.Endpoints {
			logger := slog.With("integration", st.integration.ID, "endpoint", epName)
			e.fetchAndCache(e.ctx, st, epName, ep, logger)

			st.mu.RLock()
			if cache, ok := st.endpoints[epName]; ok && cache.err != nil {
				allOK = false
			}
			st.mu.RUnlock()
		}
		if allOK {
			slog.Info("service recovered after start", "integration", st.integration.ID, "attempt", attempt)
			return
		}
	}
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
