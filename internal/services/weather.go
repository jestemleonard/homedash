package services

import (
	"log/slog"
	"sync"
	"time"

	"github.com/jestemleonard/homedash/internal/config"
	"github.com/jestemleonard/homedash/internal/models"
)

// WeatherService provides weather data with caching
type WeatherService struct {
	config   *config.WeatherConfig
	provider WeatherProvider
	cache    *models.WeatherData
	cacheMu  sync.RWMutex
	cacheExp time.Time
	cacheTTL time.Duration
}

// NewWeatherService creates a new weather service
func NewWeatherService(cfg *config.WeatherConfig) *WeatherService {
	var provider WeatherProvider

	switch cfg.Provider {
	case "openweathermap":
		provider = &OpenWeatherMapProvider{}
	case "openmeteo", "":
		provider = &OpenMeteoProvider{}
	default:
		slog.Warn("unknown weather provider, falling back to Open-Meteo", "provider", cfg.Provider)
		provider = &OpenMeteoProvider{}
	}

	return &WeatherService{
		config:   cfg,
		provider: provider,
		cacheTTL: 10 * time.Minute, // Cache weather for 10 minutes
	}
}

// GetWeather returns current weather data (with caching)
func (s *WeatherService) GetWeather() (*models.WeatherData, error) {
	// Check cache first
	s.cacheMu.RLock()
	if s.cache != nil && time.Now().Before(s.cacheExp) {
		defer s.cacheMu.RUnlock()
		return s.cache, nil
	}
	s.cacheMu.RUnlock()

	// Fetch fresh data
	weather, err := s.provider.GetWeather(s.config)
	if err != nil {
		// If we have stale cache, return it on error
		s.cacheMu.RLock()
		if s.cache != nil {
			defer s.cacheMu.RUnlock()
			slog.Warn("weather fetch failed, using stale cache", "error", err)
			return s.cache, nil
		}
		s.cacheMu.RUnlock()
		return nil, err
	}

	// Update cache
	s.cacheMu.Lock()
	s.cache = weather
	s.cacheExp = time.Now().Add(s.cacheTTL)
	s.cacheMu.Unlock()

	return weather, nil
}

// IsEnabled returns whether weather is enabled
func (s *WeatherService) IsEnabled() bool {
	return s.config != nil && s.config.Enabled
}
