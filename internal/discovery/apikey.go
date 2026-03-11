package discovery

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"

	yamlv3 "gopkg.in/yaml.v3"
)

// ConfigDiscovery describes where to find the API key inside a container's config volume.
type ConfigDiscovery struct {
	ConfigPath string `yaml:"config_path"` // e.g. "/config/config.xml"
	Format     string `yaml:"format"`      // "xml", "json", "yaml"
	Key        string `yaml:"key"`         // element/field name, e.g. "ApiKey"
}

// parseAPIKey extracts an API key from raw config file data based on the
// format and key specified in ConfigDiscovery.
func parseAPIKey(data []byte, disc *ConfigDiscovery) (string, error) {
	switch strings.ToLower(disc.Format) {
	case "xml":
		return extractFromXML(data, disc.Key)
	case "json":
		return extractFromJSON(data, disc.Key)
	case "yaml", "yml":
		return extractFromYAML(data, disc.Key)
	default:
		return "", fmt.Errorf("unsupported config format: %s", disc.Format)
	}
}

// extractFromXML finds an element by tag name and returns its char data.
func extractFromXML(data []byte, key string) (string, error) {
	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == key {
			tok, err = decoder.Token()
			if err != nil {
				return "", fmt.Errorf("reading XML element %s: %w", key, err)
			}
			if cd, ok := tok.(xml.CharData); ok {
				val := strings.TrimSpace(string(cd))
				if val != "" {
					return val, nil
				}
			}
		}
	}
	return "", fmt.Errorf("XML element %q not found", key)
}

// extractFromJSON looks up a top-level key in a JSON object.
func extractFromJSON(data []byte, key string) (string, error) {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return "", fmt.Errorf("parsing JSON: %w", err)
	}
	val, ok := obj[key]
	if !ok {
		return "", fmt.Errorf("JSON key %q not found", key)
	}
	return fmt.Sprintf("%v", val), nil
}

// extractFromYAML looks up a key in a YAML document.
// Supports dot-separated paths for nested keys (e.g. "auth.apikey").
func extractFromYAML(data []byte, key string) (string, error) {
	var obj map[string]any
	if err := yamlv3.Unmarshal(data, &obj); err != nil {
		return "", fmt.Errorf("parsing YAML: %w", err)
	}

	parts := strings.Split(key, ".")
	current := any(obj)
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return "", fmt.Errorf("YAML key %q: %q is not a map", key, part)
		}
		current, ok = m[part]
		if !ok {
			return "", fmt.Errorf("YAML key %q not found", key)
		}
	}
	return fmt.Sprintf("%v", current), nil
}
