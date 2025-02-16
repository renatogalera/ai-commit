package ai

import (
	"context"
	"regexp"
	"strings"

	"github.com/renatogalera/ai-commit/pkg/committypes"
)

// AIClient defines the interface for AI providers.
type AIClient interface {
	GetCommitMessage(ctx context.Context, prompt string) (string, error)
	SanitizeResponse(message, commitType string) string // Provider-specific sanitize
	ProviderName() string                               // Return provider name
	MaybeSummarizeDiff(diff string, maxLength int) (string, bool)
}

// BaseAIClient struct to embed common functionalities in providers
type BaseAIClient struct {
	Provider string // Provider name, e.g., "openai", "gemini"
}

func (b *BaseAIClient) ProviderName() string {
	return b.Provider
}

// SanitizeResponse cleans the AI-generated commit message. (Default implementation - can be overridden)
func (b *BaseAIClient) SanitizeResponse(message, commitType string) string {
	message = strings.ReplaceAll(message, "```", "")
	message = strings.TrimSpace(message)
	if commitType != "" {
		lines := strings.SplitN(message, "\n", 2)
		if len(lines) > 0 {
			sanitizePattern := regexp.MustCompile(`^(?:(\p{So}|\p{Sk}|:\w+:)\s*)?(` + committypes.TypesRegexPattern() + `)(\([^)]+\))?:\s*`)
			lines[0] = sanitizePattern.ReplaceAllString(lines[0], "")
		}
		message = strings.Join(lines, "\n")
	}
	return strings.TrimSpace(message)
}

// MaybeSummarizeDiff truncates the diff if it exceeds maxLength and appends a note. (Default implementation - can be overridden)
func (b *BaseAIClient) MaybeSummarizeDiff(diff string, maxLength int) (string, bool) {
	if len(diff) <= maxLength {
		return diff, false
	}
	truncated := diff[:maxLength]
	if lastNewLine := strings.LastIndex(truncated, "\n"); lastNewLine != -1 {
		truncated = truncated[:lastNewLine]
	}
	truncated += "\n[... diff truncated for brevity ...]"
	return truncated, true
}
