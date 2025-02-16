package gemini

import (
	"context"
	"fmt"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"github.com/renatogalera/ai-commit/pkg/ai"
)

type GeminiClient struct {
	ai.BaseAIClient
	client *genai.GenerativeModel
}

func NewClient(client *genai.GenerativeModel) *GeminiClient {
	return &GeminiClient{
		BaseAIClient: ai.BaseAIClient{Provider: "gemini"}, // Initialize BaseAIClient
		client:       client,
	}
}

func NewGeminiProClient(ctx context.Context, apiKey string, modelName string) (*genai.GenerativeModel, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("error creating gemini client: %w", err)
	}
	model := client.GenerativeModel(modelName)
	return model, nil
}

func (gc *GeminiClient) GetCommitMessage(ctx context.Context, prompt string) (string, error) {
	resp, err := gc.client.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from Gemini")
	}
	if text, ok := resp.Candidates[0].Content.Parts[0].(genai.Text); ok {
		return string(text), nil
	}
	return "", fmt.Errorf("unexpected response format from Gemini")
}

// SanitizeResponse cleans Gemini specific responses if needed.  Overrides default.
func (gc *GeminiClient) SanitizeResponse(message, commitType string) string {
	return gc.BaseAIClient.SanitizeResponse(message, commitType)
}

func (gc *GeminiClient) MaybeSummarizeDiff(diff string, maxLength int) (string, bool) {
	return gc.BaseAIClient.MaybeSummarizeDiff(diff, maxLength)
}

var _ ai.AIClient = (*GeminiClient)(nil)
