package discovery

import (
	"path/filepath"
	"strings"
)

// matchImage checks if a container image matches any of the given patterns.
// It strips tags (e.g. ":latest") before matching and supports exact match
// and glob wildcards via filepath.Match.
func matchImage(containerImage string, patterns []string) bool {
	// Strip tag from container image
	image := stripTag(containerImage)

	for _, pattern := range patterns {
		pattern = stripTag(pattern)

		// Exact match
		if strings.EqualFold(image, pattern) {
			return true
		}

		// Glob match
		if matched, _ := filepath.Match(pattern, image); matched {
			return true
		}

		// Also try matching just the image name (without registry prefix)
		short := shortName(image)
		if matched, _ := filepath.Match(pattern, short); matched {
			return true
		}
	}
	return false
}

// stripTag removes the tag portion from a Docker image reference.
// "linuxserver/sonarr:latest" -> "linuxserver/sonarr"
func stripTag(image string) string {
	// Handle digest references (sha256:...)
	if idx := strings.LastIndex(image, "@"); idx != -1 {
		image = image[:idx]
	}
	// Handle tag
	if idx := strings.LastIndex(image, ":"); idx != -1 {
		// Make sure we're not cutting a port in a registry URL (e.g. registry:5000/image)
		afterColon := image[idx+1:]
		if !strings.Contains(afterColon, "/") {
			image = image[:idx]
		}
	}
	return image
}

// shortName returns just the last path component of an image.
// "linuxserver/sonarr" -> "sonarr", "ghcr.io/org/sonarr" -> "sonarr"
func shortName(image string) string {
	if idx := strings.LastIndex(image, "/"); idx != -1 {
		return image[idx+1:]
	}
	return image
}
