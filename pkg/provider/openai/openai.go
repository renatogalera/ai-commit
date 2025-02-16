package openai

import (
	"context"
	"errors"
	"fmt"
	"strings"

	gogpt "github.com/sashabaranov/go-openai"

	"github.com/renatogalera/ai-commit/pkg/ai"
)

type OpenAIClient struct {
	ai.BaseAIClient // Embed base client
	client          *gogpt.Client
	model           string
}

// NewOpenAIClient creates a new OpenAIClient using the provided API key and model.
func NewOpenAIClient(key, model string) *OpenAIClient {
	return &OpenAIClient{
		BaseAIClient: ai.BaseAIClient{Provider: "openai"}, // Initialize base client
		client:       gogpt.NewClient(key),
		model:        model,
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

// SanitizeResponse cleans OpenAI's specific responses (if needed). Overrides default.
func (oc *OpenAIClient) SanitizeResponse(message, commitType string) string {
	// Add OpenAI-specific sanitization logic here, if different from default.
	return oc.BaseAIClient.SanitizeResponse(message, commitType) // Fallback to default if no specific logic
}

// MaybeSummarizeDiff implements provider-specific diff summarization for OpenAI (if needed). Overrides default.
func (oc *OpenAIClient) MaybeSummarizeDiff(diff string, maxLength int) (string, bool) {
	// Add OpenAI-specific summarization logic if needed.
	return oc.BaseAIClient.MaybeSummarizeDiff(diff, maxLength) // Fallback to default if no specific logic
}

var _ ai.AIClient = (*OpenAIClient)(nil)
