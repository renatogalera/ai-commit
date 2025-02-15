package anthropic

import (
	"context"
	"errors"
	"fmt"
	"strings"

	anthropicSDK "github.com/liushuangls/go-anthropic/v2"
	"github.com/renatogalera/ai-commit/pkg/ai"
)

// AnthropicClient implements the ai.AIClient interface using the Anthropic Claude API.
type AnthropicClient struct {
	client *anthropicSDK.Client
	model  string
}

// NewAnthropicClient creates a new AnthropicClient with the provided API key and model.
func NewAnthropicClient(apiKey, model string) (*AnthropicClient, error) {
	if apiKey == "" {
		return nil, errors.New("Anthropic API key is required")
	}
	// Create a new client using the go-anthropic library.
	client := anthropicSDK.NewClient(apiKey)
	if client == nil {
		return nil, errors.New("failed to create Anthropic client")
	}
	return &AnthropicClient{
		client: client,
		model:  model,
	}, nil
}

// GetCommitMessage sends the prompt to Anthropic and returns the generated commit message.
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

var _ ai.AIClient = (*AnthropicClient)(nil)
