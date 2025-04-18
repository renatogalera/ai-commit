package ollama

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/ollama/ollama/api"
	"github.com/renatogalera/ai-commit/pkg/ai"
)

type OllamaClient struct {
	ai.BaseAIClient
	client *api.Client
	model  string
}

func NewOllamaClient(baseURL, model string) *OllamaClient {
	u, _ := url.Parse(baseURL)
	client := api.NewClient(u, http.DefaultClient)
	return &OllamaClient{
		BaseAIClient: ai.BaseAIClient{Provider: "ollama"},
		client:       client,
		model:        model,
	}
}

func (oc *OllamaClient) GetCommitMessage(ctx context.Context, prompt string) (string, error) {
	stream := false
	req := &api.GenerateRequest{
		Model:  oc.model,
		Prompt: prompt,
		Stream: &stream,
	}

	var response string
	err := oc.client.Generate(ctx, req, func(resp api.GenerateResponse) error {
		response = resp.Response
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate response from Ollama: %w", err)
	}

	if response == "" {
		return "", errors.New("empty response from Ollama")
	}

	return strings.TrimSpace(response), nil
}

func (oc *OllamaClient) SanitizeResponse(message, commitType string) string {
	return oc.BaseAIClient.SanitizeResponse(message, commitType)
}

func (oc *OllamaClient) MaybeSummarizeDiff(diff string, maxLength int) (string, bool) {
	return oc.BaseAIClient.MaybeSummarizeDiff(diff, maxLength)
}

var _ ai.AIClient = (*OllamaClient)(nil) 