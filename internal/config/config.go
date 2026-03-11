package config

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration structure
type Config struct {
	Server    ServerConfig             `yaml:"server"`
	HomePage  HomePageConfig           `yaml:"home_page"`
	Search    SearchConfig             `yaml:"search"`
	Weather   WeatherConfig            `yaml:"weather"`
	Bookmarks BookmarksConfig          `yaml:"bookmarks"`
	Theme     ThemeConfig              `yaml:"theme"`
	Services     map[string]ServiceConfig `yaml:"services"`
	ServiceOrder []string                 `yaml:"-"` // populated from HOMEDASH_SERVICE_ORDER env var
	Exclude      []string                 `yaml:"exclude"`
}

// HomePageConfig controls the first page (search + weather).
// When disabled, the services page becomes the landing page and weather is skipped.
type HomePageConfig struct {
	Enabled bool `yaml:"enabled"`
}

// BookmarksConfig contains bookmarks settings
type BookmarksConfig struct {
	Enabled bool       `yaml:"enabled"`
	Items   []Bookmark `yaml:"items"`
}

// Bookmark represents a single bookmark
type Bookmark struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
	Icon string `yaml:"icon"`
}

// ServerConfig contains server settings
type ServerConfig struct {
	Port         int    `yaml:"port"`
	Host         string `yaml:"host"`
	Hostname     string `yaml:"hostname"` // global hostname for discovered service URLs
	TemplatesDir string `yaml:"templates_dir"`
	StaticDir    string `yaml:"static_dir"`
}

// SearchConfig contains search settings
type SearchConfig struct {
	Placeholder string   `yaml:"placeholder"`
	Engines     []Engine `yaml:"engines"`
}

// Engine represents a search engine configuration
type Engine struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
	Icon string `yaml:"icon"`
}

// WeatherConfig contains weather settings
type WeatherConfig struct {
	Enabled   bool    `yaml:"enabled"`
	Provider  string  `yaml:"provider"`  // "openmeteo" (default, free) or "openweathermap"
	APIKey    string  `yaml:"api_key"`   // Required for openweathermap
	Location  string  `yaml:"location"`  // City name for display and geocoding
	Latitude  float64 `yaml:"latitude"`  // Optional: if set, skips geocoding
	Longitude float64 `yaml:"longitude"` // Optional: if set, skips geocoding
	Units     string  `yaml:"units"`     // "metric" or "imperial"
}

// ThemeConfig contains theme settings
type ThemeConfig struct {
	PrimaryColor   string `yaml:"primary_color"`
	AccentColor    string `yaml:"accent_color"`
	BackgroundType string `yaml:"background_type"`
	BackgroundURL  string `yaml:"background_url"`
}

// ServiceConfig contains service/integration settings
type ServiceConfig struct {
	URL         string `yaml:"url"`
	ExternalURL string `yaml:"external_url"` // per-service override for the display URL
	APIKey      string `yaml:"api_key"`
	Icon        string `yaml:"icon"`
	ContainerID string `yaml:"-"` // runtime only, not persisted
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		HomePage: HomePageConfig{
			Enabled: true,
		},
		Server: ServerConfig{
			Port:         8080,
			Host:         "0.0.0.0",
			TemplatesDir: "web/templates",
			StaticDir:    "web/static",
		},
		Search: SearchConfig{
			Placeholder: "Search...",
			Engines: []Engine{
				{Name: "Google", URL: "https://www.google.com/search?q=", Icon: "google"},
				{Name: "DuckDuckGo", URL: "https://duckduckgo.com/?q=", Icon: "duckduckgo"},
			},
		},
		Weather: WeatherConfig{
			Enabled:  true,
			Provider: "openmeteo",
			Location: "London",
			Units:    "metric",
		},
		Theme: ThemeConfig{
			PrimaryColor:   "#1a1a2e",
			AccentColor:    "#4a9eff",
			BackgroundType: "solid",
		},
	}
}

// Load reads configuration from a YAML file
func Load(path string) (*Config, error) {
	config := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return nil, err
	}

	// Expand environment variables in the YAML content
	expanded := ExpandEnvVars(string(data))

	if err := yaml.Unmarshal([]byte(expanded), config); err != nil {
		return nil, err
	}

	// Allow excluding services via a comma-separated env var (for docker-compose).
	if env := os.Getenv("HOMEDASH_EXCLUDE"); env != "" {
		for _, s := range strings.Split(env, ",") {
			if v := strings.TrimSpace(s); v != "" {
				config.Exclude = append(config.Exclude, v)
			}
		}
	}

	// Allow ordering services via a comma-separated env var (for docker-compose).
	if env := os.Getenv("HOMEDASH_SERVICE_ORDER"); env != "" {
		for _, s := range strings.Split(env, ",") {
			if v := strings.TrimSpace(s); v != "" {
				config.ServiceOrder = append(config.ServiceOrder, v)
			}
		}
	}

	return config, nil
}

