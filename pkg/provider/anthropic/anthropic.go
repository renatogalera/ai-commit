package anthropic

import (
	"context"
	"errors"
	"fmt"
	"strings"

	anthropicSDK "github.com/liushuangls/go-anthropic/v2"
	"github.com/renatogalera/ai-commit/pkg/ai"
)

type AnthropicClient struct {
	ai.BaseAIClient
	client *anthropicSDK.Client
	model  string
}

func NewAnthropicClient(apiKey, model string) (*AnthropicClient, error) {
	if apiKey == "" {
		return nil, errors.New("anthropic API key is required")
	}
	// Create a new client using the go-anthropic library.
	client := anthropicSDK.NewClient(apiKey)
	if client == nil {
		return nil, errors.New("failed to create Anthropic client")
	}
	return &AnthropicClient{
		BaseAIClient: ai.BaseAIClient{Provider: "anthropic"},
		client:       client,
		model:        model,
	}, nil
}

func (ac *AnthropicClient) GetCommitMessage(ctx context.Context, prompt string) (string, error) {
	req := anthropicSDK.MessagesRequest{
		Model: anthropicSDK.Model(ac.model),
		Messages: []anthropicSDK.Message{
			anthropicSDK.NewUserTextMessage(prompt),
		},
		MaxTokens: 1000,
	}
	resp, err := ac.client.CreateMessages(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get message from Anthropic: %w", err)
	}
	if len(resp.Content) == 0 {
		return "", errors.New("no response from Anthropic")
	}
	// Trim and return the text from the first content block.
	msg := strings.TrimSpace(resp.Content[0].GetText())
	if msg == "" {
		return "", errors.New("empty response from Anthropic")
	}
	return msg, nil
}

// SanitizeResponse for Anthropic. Override default if needed.
func (ac *AnthropicClient) SanitizeResponse(message, commitType string) string {
	return ac.BaseAIClient.SanitizeResponse(message, commitType)
}

func (ac *AnthropicClient) MaybeSummarizeDiff(diff string, maxLength int) (string, bool) {
	return ac.BaseAIClient.MaybeSummarizeDiff(diff, maxLength)
}

var _ ai.AIClient = (*AnthropicClient)(nil)
