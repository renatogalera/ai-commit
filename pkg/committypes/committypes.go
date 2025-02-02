package committypes

import (
	"regexp"
	"strings"
)

// validTypes is the list of allowed commit types.
var validTypes = []string{
	"feat", "fix", "docs", "style", "refactor", "test", "chore", "perf", "build", "ci",
}

// BuildRegexPatternWithEmoji returns a precompiled regex that matches
// the Conventional Commit type with an optional emoji and scope.
func BuildRegexPatternWithEmoji() *regexp.Regexp {
	// Build a regex that matches:
	// - an optional emoji or gitmoji in the form of a unicode symbol or :emoji:
	// - followed by one of the valid commit types (e.g., feat, fix, etc.)
	// - optionally a scope in parentheses
	// - and a colon at the end.
	pattern := `^((\p{So}|\p{Sk}|:\w+:)\s*)?(` + strings.Join(validTypes, "|") + `)(\([^)]+\))?:`
	return regexp.MustCompile(pattern)
}

// IsValidCommitType checks if the provided commit type is in the list of valid types.
func IsValidCommitType(t string) bool {
	for _, vt := range validTypes {
		if t == vt {
			return true
		}
	}
	return false
}

// AllTypes returns the list of valid conventional commit types.
func AllTypes() []string {
	return validTypes
}

// TypesRegexPattern returns a string that can be used to match valid commit types in a regex.
func TypesRegexPattern() string {
	return strings.Join(validTypes, "|")
}
