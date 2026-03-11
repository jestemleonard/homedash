package config

import (
	"os"
	"regexp"
	"strings"
)

var envPattern = regexp.MustCompile(`\$\{([^}:]+)(?::-([^}]*))?\}`)

// ExpandEnvVars expands ${VAR_NAME} and ${VAR_NAME:-default} patterns in a string
// with environment variable values.
func ExpandEnvVars(s string) string {
	return envPattern.ReplaceAllStringFunc(s, func(match string) string {
		submatch := envPattern.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match
		}
		varName := submatch[1]
		defaultVal := ""
		if len(submatch) >= 3 {
			defaultVal = submatch[2]
		}
		if value, exists := os.LookupEnv(varName); exists {
			return value
		}
		// If the :- syntax was used, return the default (even if empty).
		// Without this, ${VAR:-} with an unset VAR would return the literal
		// "${VAR:-}" instead of an empty string.
		if strings.Contains(match, ":-") {
			return defaultVal
		}
		return match
	})
}
