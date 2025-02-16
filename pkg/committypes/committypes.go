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

// GuessCommitType tenta identificar o tipo de commit a partir do conteúdo da mensagem.
// Caso a mensagem contenha palavras-chave que indiquem uma feature, bugfix, etc.,
// retorna o tipo correspondente; caso contrário, retorna uma string vazia.
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
