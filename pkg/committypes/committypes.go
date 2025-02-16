package committypes

import (
	"regexp"
	"strings"

	"github.com/renatogalera/ai-commit/pkg/config"
)

type commitTypeInfo struct {
	Type  string
	Emoji string
}

var commitTypeList []commitTypeInfo

// InitCommitTypes now accepts []config.CommitTypeConfig directly
func InitCommitTypes(cfgTypes []config.CommitTypeConfig) {
	commitTypeList = []commitTypeInfo{}

	for _, t := range cfgTypes {
		// Copy each config.CommitTypeConfig into our internal slice
		commitTypeList = append(commitTypeList, commitTypeInfo{
			Type:  strings.TrimSpace(t.Type),
			Emoji: strings.TrimSpace(t.Emoji),
		})
	}
}

// IsValidCommitType returns true if the given commit type is in our list.
func IsValidCommitType(t string) bool {
	for _, info := range commitTypeList {
		if info.Type == t {
			return true
		}
	}
	return false
}

// GetEmojiForType returns the emoji associated with the given commit type, if any.
func GetEmojiForType(t string) string {
	for _, info := range commitTypeList {
		if info.Type == t {
			return info.Emoji
		}
	}
	return ""
}

// GuessCommitType attempts to guess a commit type by checking if the AI-generated
// message contains any known type in commitTypeList.
func GuessCommitType(message string) string {
	lower := strings.ToLower(message)
	for _, info := range commitTypeList {
		if info.Type == "" {
			continue
		}
		if strings.Contains(lower, info.Type) {
			return info.Type
		}
	}
	return ""
}

// TypesRegexPattern returns a regex pattern that matches any configured commit type.
func TypesRegexPattern() string {
	if len(commitTypeList) == 0 {
		// fallback if user has no commit types in config
		return "feat|fix|docs|style|refactor|test|chore|perf|build|ci"
	}
	var t []string
	for _, info := range commitTypeList {
		if info.Type != "" {
			t = append(t, info.Type)
		}
	}
	return strings.Join(t, "|")
}

// BuildRegexPatternWithEmoji constructs a dynamic regex pattern using
// the list of commit types. e.g.: ^((\p{So}|\p{Sk}|:\w+:)\s*)?(feat|fix|docs):
func BuildRegexPatternWithEmoji() *regexp.Regexp {
	pattern := `^((\p{So}|\p{Sk}|:\w+:)\s*)?(` + TypesRegexPattern() + `)(\([^)]+\))?:`
	return regexp.MustCompile(pattern)
}

// OPTIONAL: Provide a helper to return all commit types as []string for TUI menus, etc.
func GetAllTypes() []string {
	var results []string
	for _, info := range commitTypeList {
		results = append(results, info.Type)
	}
	return results
}
