package google

import (
	"context"
	"fmt"

	"google.golang.org/genai"

	"github.com/renatogalera/ai-commit/pkg/ai"
)

type GoogleClient struct {
	ai.BaseAIClient
	client *genai.Client
	model  string
}

func NewGoogleClient(ctx context.Context, provider, apiKey, model, baseURL string) (*GoogleClient, error) {
	cfg := &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	}
	if baseURL != "" {
		cfg.HTTPOptions.BaseURL = baseURL
	}
	client, err := genai.NewClient(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("error creating google client: %w", err)
	}
	return &GoogleClient{
		BaseAIClient: ai.BaseAIClient{Provider: provider},
		client:       client,
		model:        model,
	}, nil
}

func (gc *GoogleClient) GetCommitMessage(ctx context.Context, prompt string) (string, error) {
	resp, err := gc.client.Models.GenerateContent(ctx, gc.model, genai.Text(prompt), nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}
	text := resp.Text()
	if text == "" {
		return "", fmt.Errorf("no response from Google")
	}
	return text, nil
}

func (gc *GoogleClient) SanitizeResponse(message, commitType string) string {
	return gc.BaseAIClient.SanitizeResponse(message, commitType)
}

func (gc *GoogleClient) MaybeSummarizeDiff(diff string, maxLength int) (string, bool) {
	return gc.BaseAIClient.MaybeSummarizeDiff(diff, maxLength)
}

var _ ai.AIClient = (*GoogleClient)(nil)
