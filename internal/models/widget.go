package models

// WidgetSize represents the display size of a widget
type WidgetSize string

const (
	WidgetSizeNormal WidgetSize = "normal"
)

// HealthStatus represents the health state of a service
type HealthStatus string

const (
	HealthStatusHealthy HealthStatus = "healthy"
	HealthStatusWarning HealthStatus = "warning"
	HealthStatusError   HealthStatus = "error"
	HealthStatusOff     HealthStatus = "off"
)

// Widget represents a dashboard widget
type Widget struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	Icon             string       `json:"icon,omitempty"`
	URL              string       `json:"url,omitempty"`
	Running          bool         `json:"running"`
	Health           HealthStatus `json:"health"`
	Error            string       `json:"error,omitempty"`
	Warning          string       `json:"warning,omitempty"`
	Integration      string       `json:"integration,omitempty"`
	IntegrationName  string       `json:"integration_name,omitempty"`
	IntegrationColor string       `json:"integration_color,omitempty"`
	Fields           []WidgetField `json:"fields,omitempty"`
}

// ServiceGroup groups widgets belonging to the same integration/service.
type ServiceGroup struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Icon        string       `json:"icon,omitempty"`
	Color       string       `json:"color,omitempty"`
	URL         string       `json:"url,omitempty"`
	Health      HealthStatus `json:"health"`
	ContainerID string       `json:"container_id,omitempty"`
	Widgets     []Widget     `json:"widgets"`
}

// WidgetField represents a field within a widget
type WidgetField struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Type  string `json:"type,omitempty"`
}
