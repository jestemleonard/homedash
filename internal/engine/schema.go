package engine

import (
	"fmt"
	"strings"
	"time"

	"github.com/jestemleonard/homedash/internal/discovery"

	"gopkg.in/yaml.v3"
)

// IntegrationDef is the top-level structure for an integration YAML file.
type IntegrationDef struct {
	ID        string       `yaml:"id"`
	Name      string       `yaml:"name"`
	Icon      string       `yaml:"icon"`
	Color     string       `yaml:"color"`
	Detection DetectionDef `yaml:"detection"`
	Auth      AuthDef      `yaml:"auth"`
	API       APIDef       `yaml:"api"`
	Widgets   []WidgetDef  `yaml:"widgets"`
	Actions   []ActionDef  `yaml:"actions"`
}

// DetectionDef describes how to auto-detect the service.
type DetectionDef struct {
	Images          []string                  `yaml:"images"`
	DefaultPort     int                       `yaml:"default_port"`
	ConfigDiscovery *discovery.ConfigDiscovery `yaml:"config_discovery,omitempty"`
}

// AuthDef describes the authentication method for the service API.
type AuthDef struct {
	Type   string        `yaml:"type"` // none, api_key, custom
	APIKey *APIKeyAuth   `yaml:"api_key,omitempty"`
	Custom []CustomEntry `yaml:"custom,omitempty"`
}

// APIKeyAuth configures API key authentication.
type APIKeyAuth struct {
	Location string `yaml:"location"` // header or query
	Name     string `yaml:"name"`
}

// CustomEntry is a custom header sent with every request.
type CustomEntry struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

// APIDef describes the API structure of the service.
type APIDef struct {
	BasePath  string                 `yaml:"base_path"`
	Endpoints map[string]EndpointDef `yaml:"endpoints"`
}

// EndpointDef describes a single API endpoint.
type EndpointDef struct {
	Path     string         `yaml:"path"`
	Method   string         `yaml:"method"`
	Interval Duration       `yaml:"interval"`
	Params   map[string]any `yaml:"params"`
	Mapping  map[string]any `yaml:"mapping"`
}

// WidgetDef describes a widget produced by the integration.
type WidgetDef struct {
	ID       string        `yaml:"id"`
	Name     string        `yaml:"name"`
	Endpoint string        `yaml:"endpoint"`
	Fields   OrderedFields `yaml:"fields"`
}

// OrderedFields preserves YAML map key order for widget fields.
// Regular map[string]FieldDef would randomise field display order on each render.
type OrderedFields struct {
	keys   []string
	fields map[string]FieldDef
}

// Keys returns field names in the order they appear in the YAML definition.
func (o OrderedFields) Keys() []string { return o.keys }

// Get returns the FieldDef for the given key.
func (o OrderedFields) Get(name string) (FieldDef, bool) {
	fd, ok := o.fields[name]
	return fd, ok
}

// Len returns the number of fields.
func (o OrderedFields) Len() int { return len(o.keys) }

// UnmarshalYAML reads the mapping node in document order.
func (o *OrderedFields) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("fields: expected a mapping, got node kind %v", value.Kind)
	}
	o.fields = make(map[string]FieldDef, len(value.Content)/2)
	for i := 0; i+1 < len(value.Content); i += 2 {
		key := value.Content[i].Value
		var fd FieldDef
		if err := value.Content[i+1].Decode(&fd); err != nil {
			return fmt.Errorf("field %q: %w", key, err)
		}
		o.keys = append(o.keys, key)
		o.fields[key] = fd
	}
	return nil
}

// FieldDef describes a field within a widget.
type FieldDef struct {
	Label   string `yaml:"label"`
	Display string `yaml:"display"`
}

// ActionDef describes an action that can be triggered on the service.
type ActionDef struct {
	ID       string `yaml:"id"`
	Endpoint string `yaml:"endpoint"`
	Method   string `yaml:"method"`
	Body     any    `yaml:"body"`
}

// Duration wraps time.Duration with YAML "30s"/"5m" unmarshalling.
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	s = strings.TrimSpace(s)
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	d.Duration = parsed
	return nil
}

// ServiceCredentials holds credentials for a configured service.
type ServiceCredentials struct {
	APIKey string
}
