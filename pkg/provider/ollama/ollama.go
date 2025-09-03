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

func NewOllamaClient(provider, baseURL, model string) (*OllamaClient, error) {
    u, err := url.Parse(strings.TrimSpace(baseURL))
    if err != nil || u.Scheme == "" || u.Host == "" {
        return nil, fmt.Errorf("invalid Ollama baseURL: %q", baseURL)
    }
    if strings.TrimSpace(model) == "" {
        return nil, fmt.Errorf("ollama model is required")
    }
    client := api.NewClient(u, http.DefaultClient)
    return &OllamaClient{
        BaseAIClient: ai.BaseAIClient{Provider: provider},
        client:       client,
        model:        model,
    }, nil
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
		return "", fmt.Errorf("ollama generate failed: %w", err)
	}
	if strings.TrimSpace(response) == "" {
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

func pick(s, dft string) string {
	if strings.TrimSpace(s) != "" {
		return s
	}
	return dft
}

var _ ai.AIClient = (*OllamaClient)(nil)
