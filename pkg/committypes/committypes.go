package committypes

import (
	"regexp"
	"strings"
)

// validTypes is the list of allowed commit types.
var validTypes = []string{
	"feat", "fix", "docs", "style", "refactor", "test", "chore", "perf", "build", "ci",
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

// TypesRegexPattern joins all valid types for use in a regex pattern.
func TypesRegexPattern() string {
	return strings.Join(validTypes, "|")
}

// BuildRegexPatternWithEmoji builds a regex pattern that matches a conventional commit
// type with an optional emoji and optional scope before the colon.
//
// Example commits that will match:
//
//	"feat: Something"
//	"feat(README): Something"
//	"✨ feat: Something"
//	"🐛 fix(core): Something"
func BuildRegexPatternWithEmoji() *regexp.Regexp {
	// Explanation of capture groups:
	// 1) ^((\p{So}|\p{Sk}|:\w+:)\s*)?  = optional emoji
	// 2) (feat|fix|docs|style|...)     = the recognized commit type
	// 3) (\([^)]+\))?                  = optional (scope)
	// 4) :                             = literal colon
	pattern := `^((\p{So}|\p{Sk}|:\w+:)\s*)?(` + TypesRegexPattern() + `)(\([^)]+\))?:`
	return regexp.MustCompile(pattern)
}
