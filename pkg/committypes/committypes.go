package committypes

import (
	"regexp"
	"strings"
)

var validTypes = []string{
	"feat", "fix", "docs", "style", "refactor", "test", "chore", "perf", "build", "ci",
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
