package openai

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	gogpt "github.com/sashabaranov/go-openai"

	"github.com/renatogalera/ai-commit/pkg/ai"
	"github.com/renatogalera/ai-commit/pkg/committypes"
)

type OpenAIClient struct {
	client *gogpt.Client
	model  string
}

// NewOpenAIClient creates a new OpenAIClient using the provided client and model.
func NewOpenAIClient(client *gogpt.Client, model string) *OpenAIClient {
	return &OpenAIClient{
		client: client,
		model:  model,
	}
}

func (oc *OpenAIClient) GetCommitMessage(ctx context.Context, prompt string) (string, error) {
	req := gogpt.ChatCompletionRequest{
		Model: oc.model,
		Messages: []gogpt.ChatCompletionMessage{
			{
				Role:    gogpt.ChatMessageRoleUser,
				Content: prompt,
			},
		},
	}
	resp, err := oc.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get chat completion: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("no response from OpenAI")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
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

func SanitizeOpenAIResponse(msg, commitType string) string {
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

var _ ai.AIClient = (*OpenAIClient)(nil)
