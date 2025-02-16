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

var _ ai.AIClient = (*OpenAIClient)(nil)
