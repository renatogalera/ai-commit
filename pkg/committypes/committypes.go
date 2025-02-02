package committypes

import (
	"regexp"
	"strings"
)

var validTypes = []string{
	"feat", "fix", "docs", "style", "refactor", "test", "chore", "perf", "build", "ci",
}

// IsValidCommitType checks if the provided commit type is in the known list.
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

// BuildRegexPatternWithEmoji builds a regex pattern that matches conventional commit messages with emoji.
func BuildRegexPatternWithEmoji() *regexp.Regexp {
	pattern := `^((\p{So}|\p{Sk}|:\w+:)\s*)?(` + TypesRegexPattern() + `):`
	return regexp.MustCompile(pattern)
}
