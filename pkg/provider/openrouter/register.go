package openrouter

import (
	"context"

	"github.com/renatogalera/ai-commit/pkg/ai"
	"github.com/renatogalera/ai-commit/pkg/config"
	compat "github.com/renatogalera/ai-commit/pkg/provider/openai_compat"
	"github.com/renatogalera/ai-commit/pkg/provider/registry"
)

const ProviderName = "openrouter"

func factory(ctx context.Context, name string, ps config.ProviderSettings) (ai.AIClient, error) {
    // OpenRouter is OpenAI-compatible; reuse the compat client.
    return compat.NewCompatClient(name, ps.APIKey, ps.Model, ps.BaseURL), nil
}

func init() {
    registry.Register(ProviderName, factory)
    registry.RegisterDefaults(ProviderName, config.ProviderSettings{Model: "openrouter/auto", BaseURL: "https://openrouter.ai/api/v1"})
    registry.SetRequiresAPIKey(ProviderName, true)
}
