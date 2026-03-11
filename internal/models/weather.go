package models

// WeatherData contains weather information
type WeatherData struct {
	Location    string  `json:"location"`
	Temperature float64 `json:"temperature"`
	Condition   string  `json:"condition"`
	Icon        string  `json:"icon"`
	Humidity    int     `json:"humidity"`
	WindSpeed   float64 `json:"wind_speed"`
	FeelsLike   float64 `json:"feels_like"`
}
