package committypes

import (
	"regexp"
	"sort"
	"strings"

	"github.com/renatogalera/ai-commit/pkg/config"
)

type commitTypeInfo struct {
	Type  string
	Emoji string
}

var commitTypeList []commitTypeInfo

// InitCommitTypes resets the known commit type list.
func InitCommitTypes(cfgTypes []config.CommitTypeConfig) {
	commitTypeList = commitTypeList[:0]
	for _, t := range cfgTypes {
		commitTypeList = append(commitTypeList, commitTypeInfo{
			Type:  strings.TrimSpace(t.Type),
			Emoji: strings.TrimSpace(t.Emoji),
		})
	}
}

// IsValidCommitType returns true if t is in the configured list.
func IsValidCommitType(t string) bool {
	for _, info := range commitTypeList {
		if info.Type == t {
			return true
		}
	}
	return false
}

func GetEmojiForType(t string) string {
	for _, info := range commitTypeList {
		if info.Type == t {
			return info.Emoji
		}
	}
	return ""
}

// GuessCommitType tries to pick the most likely type from the message's first line.
// It uses word-boundary matching to avoid "fix" in "prefix" false-positives.
func GuessCommitType(message string) string {
	line := strings.ToLower(strings.TrimSpace(firstLine(message)))
	types := GetAllTypes()

	// Prefer longer types first (e.g., "refactor" before "feat")
	sort.SliceStable(types, func(i, j int) bool { return len(types[i]) > len(types[j]) })

	for _, t := range types {
		if t == "" {
			continue
		}
		pat := regexp.MustCompile(`\b` + regexp.QuoteMeta(strings.ToLower(t)) + `\b`)
		if pat.FindStringIndex(line) != nil {
			return t
		}
	}
	return ""
}

// TypesRegexPattern builds a safe alternation for all configured types.
func TypesRegexPattern() string {
	if len(commitTypeList) == 0 {
		return "feat|fix|docs|style|refactor|test|chore|perf|build|ci"
	}
	var t []string
	for _, info := range commitTypeList {
		if info.Type != "" {
			t = append(t, regexp.QuoteMeta(info.Type))
		}
	}
	return strings.Join(t, "|")
}

// BuildRegexPatternWithEmoji matches optional emoji, a valid type, optional scope, and colon.
func BuildRegexPatternWithEmoji() *regexp.Regexp {
	pattern := `^((\p{So}|\p{Sk}|:\w+:)\s*)?(` + TypesRegexPattern() + `)(\([^)]+\))?:\s*`
	return regexp.MustCompile(pattern)
}

func GetAllTypes() []string {
	var results []string
	for _, info := range commitTypeList {
		results = append(results, info.Type)
	}
	return results
}

func firstLine(msg string) string {
	lines := strings.Split(msg, "\n")
	return strings.TrimSpace(lines[0])
}
