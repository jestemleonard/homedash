package engine

import (
	"fmt"

	"github.com/jestemleonard/homedash/internal/models"
)

// BuildWidget constructs a models.Widget from an integration definition,
// widget definition, extracted data, and service health info.
func BuildWidget(integration IntegrationDef, widgetDef WidgetDef, data map[string]any, serviceURL string, healthy bool) models.Widget {
	widgetID := fmt.Sprintf("%s_%s", integration.ID, widgetDef.ID)

	health := models.HealthStatusHealthy
	if !healthy {
		health = models.HealthStatusError
	}

	w := models.Widget{
		ID:               widgetID,
		Name:             widgetDef.Name,
		Icon:             integration.Icon,
		URL:              serviceURL,
		Running:          healthy,
		Health:           health,
		Integration:      integration.ID,
		IntegrationName:  integration.Name,
		IntegrationColor: integration.Color,
	}

	// Build fields from widget definition
	w.Fields = buildFields(widgetDef, data)

	return w
}

func buildFields(widgetDef WidgetDef, data map[string]any) []models.WidgetField {
	if widgetDef.Fields.Len() == 0 || data == nil {
		return nil
	}

	var fields []models.WidgetField
	for _, name := range widgetDef.Fields.Keys() {
		fdef, _ := widgetDef.Fields.Get(name)
		label := fdef.Label
		if label == "" {
			label = name
		}

		value := ""
		if v, ok := data[name]; ok && v != nil {
			value = fmt.Sprintf("%v", v)
		}

		fieldType := fdef.Display
		if fieldType == "" {
			fieldType = "text"
		}

		fields = append(fields, models.WidgetField{
			Label: label,
			Value: value,
			Type:  fieldType,
		})
	}

	return fields
}
