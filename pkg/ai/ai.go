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
    SanitizeResponse(message, commitType string) string
    ProviderName() string
    MaybeSummarizeDiff(diff string, maxLength int) (string, bool)
}

// StreamingAIClient is an optional interface that providers can implement
// to stream text deltas while generating the message. Implementations should
// call onDelta with incremental text (may be per-token or per-chunk) and
// return the final full text when the stream finishes.
type StreamingAIClient interface {
    StreamCommitMessage(ctx context.Context, prompt string, onDelta func(delta string)) (final string, err error)
}

type BaseAIClient struct {
	Provider string
}

func (b *BaseAIClient) ProviderName() string {
	return b.Provider
}

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
