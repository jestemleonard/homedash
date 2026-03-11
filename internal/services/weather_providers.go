package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/jestemleonard/homedash/internal/config"
	"github.com/jestemleonard/homedash/internal/models"
)

// WeatherProvider defines the interface for weather data providers
type WeatherProvider interface {
	GetWeather(cfg *config.WeatherConfig) (*models.WeatherData, error)
}

// GeoLocation holds coordinates for a location
type GeoLocation struct {
	Latitude  float64
	Longitude float64
	Name      string
}

// httpClient is a shared HTTP client with timeout
var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

// OpenMeteoProvider implements weather using Open-Meteo API (free, no key required)
type OpenMeteoProvider struct{}

// openMeteoGeoResponse is the response from Open-Meteo geocoding API
type openMeteoGeoResponse struct {
	Results []struct {
		Name      string  `json:"name"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Country   string  `json:"country"`
	} `json:"results"`
}

// openMeteoWeatherResponse is the response from Open-Meteo weather API
type openMeteoWeatherResponse struct {
	Current struct {
		Temperature  float64 `json:"temperature_2m"`
		Humidity     int     `json:"relative_humidity_2m"`
		ApparentTemp float64 `json:"apparent_temperature"`
		WeatherCode  int     `json:"weather_code"`
		WindSpeed    float64 `json:"wind_speed_10m"`
	} `json:"current"`
}

// GetWeather fetches weather from Open-Meteo
func (p *OpenMeteoProvider) GetWeather(cfg *config.WeatherConfig) (*models.WeatherData, error) {
	// Get coordinates
	lat, lon := cfg.Latitude, cfg.Longitude
	location := cfg.Location

	if lat == 0 && lon == 0 {
		// Geocode the location
		geo, err := geocodeOpenMeteo(cfg.Location)
		if err != nil {
			return nil, fmt.Errorf("geocoding failed: %w", err)
		}
		lat, lon = geo.Latitude, geo.Longitude
		if geo.Name != "" {
			location = geo.Name
		}
	}

	// Build weather API URL
	units := "celsius"
	windUnit := "kmh"
	if cfg.Units == "imperial" {
		units = "fahrenheit"
		windUnit = "mph"
	}

	apiURL := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f&current=temperature_2m,relative_humidity_2m,apparent_temperature,weather_code,wind_speed_10m&temperature_unit=%s&wind_speed_unit=%s",
		lat, lon, units, windUnit,
	)

	resp, err := httpClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("weather API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather API returned status %d", resp.StatusCode)
	}

	var weather openMeteoWeatherResponse
	if err := json.NewDecoder(resp.Body).Decode(&weather); err != nil {
		return nil, fmt.Errorf("failed to decode weather response: %w", err)
	}

	return &models.WeatherData{
		Location:    location,
		Temperature: weather.Current.Temperature,
		Condition:   weatherCodeToCondition(weather.Current.WeatherCode),
		Icon:        weatherCodeToIcon(weather.Current.WeatherCode),
		Humidity:    weather.Current.Humidity,
		WindSpeed:   weather.Current.WindSpeed,
		FeelsLike:   weather.Current.ApparentTemp,
	}, nil
}

// geocodeOpenMeteo converts a location name to coordinates using Open-Meteo
func geocodeOpenMeteo(location string) (*GeoLocation, error) {
	apiURL := fmt.Sprintf(
		"https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1&language=en&format=json",
		url.QueryEscape(location),
	)

	resp, err := httpClient.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var geo openMeteoGeoResponse
	if err := json.NewDecoder(resp.Body).Decode(&geo); err != nil {
		return nil, err
	}

	if len(geo.Results) == 0 {
		return nil, fmt.Errorf("location not found: %s", location)
	}

	return &GeoLocation{
		Latitude:  geo.Results[0].Latitude,
		Longitude: geo.Results[0].Longitude,
		Name:      geo.Results[0].Name,
	}, nil
}

// OpenWeatherMapProvider implements weather using OpenWeatherMap API
type OpenWeatherMapProvider struct{}

// openWeatherMapResponse is the response from OpenWeatherMap API
type openWeatherMapResponse struct {
	Main struct {
		Temp      float64 `json:"temp"`
		FeelsLike float64 `json:"feels_like"`
		Humidity  int     `json:"humidity"`
	} `json:"main"`
	Weather []struct {
		ID          int    `json:"id"`
		Main        string `json:"main"`
		Description string `json:"description"`
	} `json:"weather"`
	Wind struct {
		Speed float64 `json:"speed"`
	} `json:"wind"`
	Name string `json:"name"`
}

// GetWeather fetches weather from OpenWeatherMap
func (p *OpenWeatherMapProvider) GetWeather(cfg *config.WeatherConfig) (*models.WeatherData, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("OpenWeatherMap requires an API key")
	}

	units := "metric"
	if cfg.Units == "imperial" {
		units = "imperial"
	}

	var apiURL string
	if cfg.Latitude != 0 || cfg.Longitude != 0 {
		apiURL = fmt.Sprintf(
			"https://api.openweathermap.org/data/2.5/weather?lat=%f&lon=%f&units=%s&appid=%s",
			cfg.Latitude, cfg.Longitude, units, cfg.APIKey,
		)
	} else {
		apiURL = fmt.Sprintf(
			"https://api.openweathermap.org/data/2.5/weather?q=%s&units=%s&appid=%s",
			url.QueryEscape(cfg.Location), units, cfg.APIKey,
		)
	}

	resp, err := httpClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("weather API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather API returned status %d", resp.StatusCode)
	}

	var weather openWeatherMapResponse
	if err := json.NewDecoder(resp.Body).Decode(&weather); err != nil {
		return nil, fmt.Errorf("failed to decode weather response: %w", err)
	}

	condition := "Unknown"
	icon := "cloud"
	if len(weather.Weather) > 0 {
		condition = weather.Weather[0].Main
		icon = owmIDToIcon(weather.Weather[0].ID)
	}

	return &models.WeatherData{
		Location:    weather.Name,
		Temperature: weather.Main.Temp,
		Condition:   condition,
		Icon:        icon,
		Humidity:    weather.Main.Humidity,
		WindSpeed:   weather.Wind.Speed,
		FeelsLike:   weather.Main.FeelsLike,
	}, nil
}

// weatherCodeToCondition converts Open-Meteo weather codes to human-readable conditions
func weatherCodeToCondition(code int) string {
	switch {
	case code == 0:
		return "Clear"
	case code == 1:
		return "Mainly Clear"
	case code == 2:
		return "Partly Cloudy"
	case code == 3:
		return "Overcast"
	case code >= 45 && code <= 48:
		return "Foggy"
	case code >= 51 && code <= 55:
		return "Drizzle"
	case code >= 56 && code <= 57:
		return "Freezing Drizzle"
	case code >= 61 && code <= 65:
		return "Rain"
	case code >= 66 && code <= 67:
		return "Freezing Rain"
	case code >= 71 && code <= 75:
		return "Snow"
	case code == 77:
		return "Snow Grains"
	case code >= 80 && code <= 82:
		return "Rain Showers"
	case code >= 85 && code <= 86:
		return "Snow Showers"
	case code == 95:
		return "Thunderstorm"
	case code >= 96 && code <= 99:
		return "Thunderstorm with Hail"
	default:
		return "Unknown"
	}
}

// weatherCodeToIcon converts Open-Meteo weather codes to icon names
func weatherCodeToIcon(code int) string {
	switch {
	case code == 0:
		return "sun"
	case code >= 1 && code <= 2:
		return "cloud-sun"
	case code == 3:
		return "cloud"
	case code >= 45 && code <= 48:
		return "fog"
	case code >= 51 && code <= 67:
		return "cloud-rain"
	case code >= 71 && code <= 86:
		return "snowflake"
	case code >= 95:
		return "cloud-lightning"
	default:
		return "cloud"
	}
}

// owmIDToIcon converts OpenWeatherMap weather IDs to icon names
func owmIDToIcon(id int) string {
	switch {
	case id >= 200 && id < 300:
		return "cloud-lightning"
	case id >= 300 && id < 400:
		return "cloud-rain"
	case id >= 500 && id < 600:
		return "cloud-rain"
	case id >= 600 && id < 700:
		return "snowflake"
	case id >= 700 && id < 800:
		return "fog"
	case id == 800:
		return "sun"
	case id == 801:
		return "cloud-sun"
	case id > 801:
		return "cloud"
	default:
		return "cloud"
	}
}
