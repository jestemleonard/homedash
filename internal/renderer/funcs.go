package renderer

import (
	"fmt"
	"html/template"
	"strings"

	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// TemplateFuncs returns custom template functions
func TemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"formatBytes":   formatBytes,
		"formatPercent": formatPercent,
		"formatTemp":    formatTemp,
		"lower":         strings.ToLower,
		"upper":         strings.ToUpper,
		"title":         cases.Title(language.English).String,
		"join":          strings.Join,
		"contains":      strings.Contains,
		"hasPrefix":     strings.HasPrefix,
		"hasSuffix":     strings.HasSuffix,
		"dict":          dict,
		"list":          list,
		"default":       defaultVal,
		"ternary":       ternary,
		"now":           time.Now,
		"safeHTML":      safeHTML,
		"safeCSS":       safeCSS,
	}
}

// formatBytes converts bytes to human-readable format
func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// formatPercent formats a float as a percentage
func formatPercent(p float64) string {
	return fmt.Sprintf("%.1f%%", p)
}

// formatTemp formats temperature with degree symbol
func formatTemp(t float64, unit string) string {
	symbol := "C"
	if strings.ToLower(unit) == "imperial" || strings.ToLower(unit) == "f" {
		symbol = "F"
	}
	return fmt.Sprintf("%.0f°%s", t, symbol)
}

// dict creates a map from key-value pairs for passing to templates
func dict(values ...any) map[string]any {
	if len(values)%2 != 0 {
		return nil
	}
	m := make(map[string]any, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			continue
		}
		m[key] = values[i+1]
	}
	return m
}

// list creates a slice from arguments
func list(values ...any) []any {
	return values
}

// defaultVal returns the default value if the first is empty/nil
func defaultVal(def, val any) any {
	if val == nil {
		return def
	}
	if s, ok := val.(string); ok && s == "" {
		return def
	}
	return val
}

// ternary returns a if condition is true, b otherwise
func ternary(condition bool, a, b any) any {
	if condition {
		return a
	}
	return b
}

// safeHTML marks a string as safe HTML (use carefully!)
func safeHTML(s string) template.HTML {
	return template.HTML(s)
}

// safeCSS marks a string as safe CSS
func safeCSS(s string) template.CSS {
	return template.CSS(s)
}
