package git

import (
	"strings"
)

// SuggestScope analyzes diff file paths and suggests a Conventional Commits scope.
// Returns "" if no meaningful scope can be determined.
func SuggestScope(diff string) string {
	lines := strings.Split(diff, "\n")
	counts := make(map[string]int)

	for _, line := range lines {
		if !strings.HasPrefix(line, "diff --git ") {
			continue
		}
		filePath := parseFilePath(line)
		if filePath == "" {
			continue
		}
		scope := scopeFromPath(filePath)
		if scope != "" {
			counts[scope]++
		}
	}

	if len(counts) == 0 {
		return ""
	}

	// Find the most frequent scope
	var bestScope string
	bestCount := 0
	for scope, count := range counts {
		if count > bestCount || (count == bestCount && len(scope) < len(bestScope)) {
			bestScope = scope
			bestCount = count
		}
	}

	// If there are 3+ scopes with equal counts, let the AI decide
	if len(counts) >= 3 {
		equalCount := 0
		for _, count := range counts {
			if count == bestCount {
				equalCount++
			}
		}
		if equalCount >= 3 {
			return ""
		}
	}

	return bestScope
}

// scopeFromPath extracts a scope name from a file path.
func scopeFromPath(filePath string) string {
	parts := strings.Split(filePath, "/")

	// Root files (no directory) have no meaningful scope
	if len(parts) <= 1 {
		return ""
	}

	switch parts[0] {
	case "cmd":
		return "cli"

	case "pkg", "internal":
		if len(parts) < 2 {
			return parts[0]
		}
		// Skip generic container directories
		if parts[1] == "provider" && len(parts) >= 3 {
			return parts[2]
		}
		return parts[1]

	default:
		return parts[0]
	}
}
