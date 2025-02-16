package committypes

import (
	"regexp"
	"strings"
)

var validTypes = []string{
	"feat", "fix", "docs", "style", "refactor", "test", "chore", "perf", "build", "ci",
}

// SetValidCommitTypes allows updating the list of valid commit types.
// This can be loaded from config.
func SetValidCommitTypes(types []string) {
	if types != nil { // Only update if the provided slice is not nil
		validTypes = types
	}
}

func BuildRegexPatternWithEmoji() *regexp.Regexp {
	pattern := `^((\p{So}|\p{Sk}|:\w+:)\s*)?(` + strings.Join(validTypes, "|") + `)(\([^)]+\))?:`
	return regexp.MustCompile(pattern)
}

func IsValidCommitType(t string) bool {
	for _, vt := range validTypes {
		if t == vt {
			return true
		}
	}
	return false
}

func AllTypes() []string {
	return validTypes
}

func TypesRegexPattern() string {
	return strings.Join(validTypes, "|")
}

func GuessCommitType(message string) string {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "feat"), strings.Contains(lower, "add"), strings.Contains(lower, "create"), strings.Contains(lower, "introduce"):
		return "feat"
	case strings.Contains(lower, "fix"):
		return "fix"
	case strings.Contains(lower, "doc"):
		return "docs"
	case strings.Contains(lower, "refactor"):
		return "refactor"
	case strings.Contains(lower, "test"):
		return "test"
	case strings.Contains(lower, "perf"):
		return "perf"
	case strings.Contains(lower, "build"):
		return "build"
	case strings.Contains(lower, "ci"):
		return "ci"
	case strings.Contains(lower, "chore"):
		return "chore"
	default:
		return ""
	}
}
