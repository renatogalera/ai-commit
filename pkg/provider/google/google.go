package google

import (
	"context"
	"fmt"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"github.com/renatogalera/ai-commit/pkg/ai"
)

type GoogleClient struct {
	ai.BaseAIClient
	client *genai.GenerativeModel
}

func NewClient(client *genai.GenerativeModel) *GoogleClient {
	return &GoogleClient{
		BaseAIClient: ai.BaseAIClient{Provider: "google"}, // Initialize BaseAIClient
		client:       client,
	}
}

func NewGoogleProClient(ctx context.Context, apiKey string, modelName string) (*genai.GenerativeModel, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("error creating google client: %w", err)
	}
	model := client.GenerativeModel(modelName)
	return model, nil
}

func (gc *GoogleClient) GetCommitMessage(ctx context.Context, prompt string) (string, error) {
	resp, err := gc.client.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from Google")
	}
	if text, ok := resp.Candidates[0].Content.Parts[0].(genai.Text); ok {
		return string(text), nil
	}
	return "", fmt.Errorf("unexpected response format from Google")
}

// SanitizeResponse cleans Google specific responses if needed.  Overrides default.
func (gc *GoogleClient) SanitizeResponse(message, commitType string) string {
	return gc.BaseAIClient.SanitizeResponse(message, commitType)
}

func (gc *GoogleClient) MaybeSummarizeDiff(diff string, maxLength int) (string, bool) {
	return gc.BaseAIClient.MaybeSummarizeDiff(diff, maxLength)
}

var _ ai.AIClient = (*GoogleClient)(nil)
