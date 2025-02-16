package ai

import (
	"context"
	"regexp"
	"strings"

	"github.com/renatogalera/ai-commit/pkg/committypes"
)

type AIClient interface {
	GetCommitMessage(ctx context.Context, prompt string) (string, error)
}

func MaybeSummarizeDiff(diff string, maxLength int) (string, bool) {
	if len(diff) <= maxLength {
		return diff, false
	}
	truncated := diff[:maxLength]
	lastNewLine := strings.LastIndex(truncated, "\n")
	if lastNewLine != -1 {
		truncated = truncated[:lastNewLine]
	}
	truncated += "\n[... diff truncated for brevity ...]"
	return truncated, true
}

func SanitizeResponse(msg, commitType string) string {
	msg = strings.ReplaceAll(msg, "```", "")
	msg = strings.TrimSpace(msg)
	if commitType != "" {
		lines := strings.SplitN(msg, "\n", 2)
		if len(lines) > 0 {
			sanitizePattern := regexp.MustCompile(`^(?:(\p{So}|\p{Sk}|:\w+:)\s*)?(` + committypes.TypesRegexPattern() + `)(\([^)]+\))?:\s*`)
			lines[0] = sanitizePattern.ReplaceAllString(lines[0], "")
		}
		msg = strings.Join(lines, "\n")
	}
	return strings.TrimSpace(msg)
}
