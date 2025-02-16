package deepseek

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/renatogalera/ai-commit/pkg/ai"
	gogpt "github.com/sashabaranov/go-openai"
)

// DeepseekClient implements the ai.AIClient interface using the OpenAI SDK.
type DeepseekClient struct {
	ai.BaseAIClient // Embed BaseAIClient
	client          *gogpt.Client
	model           string
}

func NewDeepseekClient(apiKey, model string) (*DeepseekClient, error) {
	if apiKey == "" {
		return nil, errors.New("deepseek API key is required")
	}
	if model == "" {
		return nil, errors.New("deepseek model is required")
	}

	config := gogpt.DefaultConfig(apiKey)
	config.BaseURL = "https://api.deepseek.com/v1"
	client := gogpt.NewClientWithConfig(config)

	return &DeepseekClient{
		BaseAIClient: ai.BaseAIClient{Provider: "deepseek"}, // Initialize BaseAIClient
		client:       client,
		model:        model,
	}, nil
}

// GetCommitMessage sends the prompt to DeepSeek and returns the generated commit message.
func (d *DeepseekClient) GetCommitMessage(ctx context.Context, prompt string) (string, error) {
	req := gogpt.ChatCompletionRequest{
		Model: d.model,
		Messages: []gogpt.ChatCompletionMessage{
			{
				Role:    gogpt.ChatMessageRoleUser,
				Content: prompt,
			},
		},
	}
	resp, err := d.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get chat completion from Deepseek: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("no response from Deepseek")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

// SanitizeResponse for Deepseek provider, if needed. Override default.
func (d *DeepseekClient) SanitizeResponse(message, commitType string) string {
	// Add Deepseek-specific sanitization logic here, if different from the default.
	return d.BaseAIClient.SanitizeResponse(message, commitType) // Fallback to default
}

// MaybeSummarizeDiff for Deepseek if specific behavior is required. Override default.
func (d *DeepseekClient) MaybeSummarizeDiff(diff string, maxLength int) (string, bool) {
	// Deepseek-specific summarization logic
	return d.BaseAIClient.MaybeSummarizeDiff(diff, maxLength) // Fallback to default
}

var _ ai.AIClient = (*DeepseekClient)(nil)
