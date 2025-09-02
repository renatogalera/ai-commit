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
	ai.BaseAIClient
	client *gogpt.Client
	model  string
}

func NewOpenAIClient(key, model, baseURL string) *OpenAIClient {
	cfg := gogpt.DefaultConfig(key)
	if strings.TrimSpace(baseURL) != "" {
		cfg.BaseURL = baseURL
	}
	return &OpenAIClient{
		BaseAIClient: ai.BaseAIClient{Provider: "openai"}, // Initialize base client
		client:       gogpt.NewClientWithConfig(cfg),
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

func (oc *OpenAIClient) SanitizeResponse(message, commitType string) string {
	return oc.BaseAIClient.SanitizeResponse(message, commitType)
}

func (oc *OpenAIClient) MaybeSummarizeDiff(diff string, maxLength int) (string, bool) {
	return oc.BaseAIClient.MaybeSummarizeDiff(diff, maxLength)
}

var _ ai.AIClient = (*OpenAIClient)(nil)
