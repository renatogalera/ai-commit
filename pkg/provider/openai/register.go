package openai

import (
	"context"

	"github.com/renatogalera/ai-commit/pkg/ai"
	"github.com/renatogalera/ai-commit/pkg/config"
	"github.com/renatogalera/ai-commit/pkg/provider/registry"
)

const ProviderName = "openai"

func factory(ctx context.Context, name string, ps config.ProviderSettings) (ai.AIClient, error) {
    // No ctx usage needed for OpenAI client construction.
    return NewOpenAIClient(name, ps.APIKey, ps.Model, ps.BaseURL), nil
}

func init() {
    registry.Register(ProviderName, factory)
    registry.RegisterDefaults(ProviderName, config.ProviderSettings{Model: "chatgpt-4o-latest", BaseURL: "https://api.openai.com/v1"})
    registry.SetRequiresAPIKey(ProviderName, true)
}
