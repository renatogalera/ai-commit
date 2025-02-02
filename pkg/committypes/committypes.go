package committypes

import (
	"regexp"
	"strings"
)

// validTypes is the list of allowed commit types.
var validTypes = []string{
	"feat", "fix", "docs", "style", "refactor", "test", "chore", "perf", "build", "ci",
}

// typeWithEmojiPattern precompiles the regex used to match
// an optional emoji, a valid commit type, an optional scope, and a colon.
var typeWithEmojiPattern = regexp.MustCompile(`^((\p{So}|\p{Sk}|:\w+:)\s*)?(` + strings.Join(validTypes, "|") + `)(\([^)]+\))?:`)

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

// BuildRegexPatternWithEmoji returns a precompiled regex that matches
// the Conventional Commit type with an optional emoji and scope.
func BuildRegexPatternWithEmoji() *regexp.Regexp {
	return typeWithEmojiPattern
}
