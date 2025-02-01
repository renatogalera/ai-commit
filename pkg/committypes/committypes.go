package committypes

import (
	"regexp"
	"strings"
)

var validTypes = []string{
	"feat", "fix", "docs", "style", "refactor", "test", "chore", "perf", "build", "ci",
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

func BuildRegexPatternWithEmoji() *regexp.Regexp {
	pattern := `^((\p{So}|\p{Sk}|:\w+:)\s+)?(` + TypesRegexPattern() + `):`
	return regexp.MustCompile(pattern)
}

