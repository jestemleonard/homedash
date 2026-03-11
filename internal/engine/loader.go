package engine

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadIntegrations reads all *.yaml files from dir and returns parsed definitions.
func LoadIntegrations(dir string) ([]IntegrationDef, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Warn("integrations directory not found", "dir", dir)
			return nil, nil
		}
		return nil, fmt.Errorf("reading integrations dir: %w", err)
	}

	var defs []IntegrationDef
	seen := make(map[string]string) // id -> filename

	for _, entry := range entries {
		if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".yaml") && !strings.HasSuffix(entry.Name(), ".yml")) {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			slog.Error("failed to read integration file", "file", entry.Name(), "error", err)
			continue
		}

		var def IntegrationDef
		if err := yaml.Unmarshal(data, &def); err != nil {
			slog.Error("failed to parse integration file", "file", entry.Name(), "error", err)
			continue
		}

		if errs := validateIntegration(def, entry.Name()); len(errs) > 0 {
			for _, e := range errs {
				slog.Error("integration validation error", "file", entry.Name(), "error", e)
			}
			continue
		}

		if prev, ok := seen[def.ID]; ok {
			slog.Warn("duplicate integration ID, skipping", "id", def.ID, "file", entry.Name(), "first", prev)
			continue
		}
		seen[def.ID] = entry.Name()

		defs = append(defs, def)
		slog.Info("loaded integration", "id", def.ID, "name", def.Name, "widgets", len(def.Widgets))
	}

	return defs, nil
}

var validAuthTypes = map[string]bool{
	"none": true, "api_key": true, "custom": true,
}

var validFieldDisplayTypes = map[string]bool{
	"number": true, "title": true, "subtitle": true, "badge": true,
	"progress_bar": true, "meta": true, "date": true, "checkmark": true,
	"text": true,
}

func validateIntegration(def IntegrationDef, filename string) []error {
	var errs []error
	prefix := filename + ": "

	if def.ID == "" {
		errs = append(errs, fmt.Errorf("%sid is required", prefix))
	}
	if def.Name == "" {
		errs = append(errs, fmt.Errorf("%sname is required", prefix))
	}
	if def.Auth.Type != "" && !validAuthTypes[def.Auth.Type] {
		errs = append(errs, fmt.Errorf("%sinvalid auth type %q", prefix, def.Auth.Type))
	}

	endpointIDs := make(map[string]bool)
	for id := range def.API.Endpoints {
		endpointIDs[id] = true
	}

	for i, w := range def.Widgets {
		wp := fmt.Sprintf("%swidgets[%d]: ", prefix, i)
		if w.ID == "" {
			errs = append(errs, fmt.Errorf("%sid is required", wp))
		}
		if w.Endpoint != "" && !endpointIDs[w.Endpoint] {
			errs = append(errs, fmt.Errorf("%sendpoint %q not defined in api.endpoints", wp, w.Endpoint))
		}
		for _, fname := range w.Fields.Keys() {
			fdef, _ := w.Fields.Get(fname)
			if fdef.Display != "" && !validFieldDisplayTypes[fdef.Display] {
				errs = append(errs, fmt.Errorf("%sfield %q has invalid display type %q", wp, fname, fdef.Display))
			}
		}
	}

	return errs
}
