package openai

import (
    openaic "github.com/renatogalera/ai-commit/pkg/provider/openai_compat"
)

// NewOpenAIClient returns an OpenAI-compatible client powered by the official SDK.
// It reuses the generic compat client to avoid duplication.
func NewOpenAIClient(provider, key, model, baseURL string) *openaic.Client {
    return openaic.NewCompatClient(provider, key, model, baseURL)
}
