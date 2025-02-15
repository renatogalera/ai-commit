package gemini

import (
	"context"
	"fmt"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"github.com/renatogalera/ai-commit/pkg/ai"
)

type GeminiClient struct {
	client *genai.GenerativeModel
}

// NewClient creates a new GeminiClient from the provided generative model.
func NewClient(client *genai.GenerativeModel) *GeminiClient {
	return &GeminiClient{client: client}
}

// NewGeminiProClient creates a new Gemini generative model client using the provided API key and model name.
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

var _ ai.AIClient = (*GeminiClient)(nil)
