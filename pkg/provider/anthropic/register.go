package anthropic

import (
	"context"

	"github.com/renatogalera/ai-commit/pkg/ai"
	"github.com/renatogalera/ai-commit/pkg/config"
	"github.com/renatogalera/ai-commit/pkg/provider/registry"
)

const ProviderName = "anthropic"

func factory(ctx context.Context, name string, ps config.ProviderSettings) (ai.AIClient, error) {
    return NewAnthropicClient(name, ps.APIKey, ps.Model, ps.BaseURL)
}

func init() {
    registry.Register(ProviderName, factory)
    registry.RegisterDefaults(ProviderName, config.ProviderSettings{Model: "claude-3-7-sonnet-latest", BaseURL: "https://api.anthropic.com/v1"})
    registry.SetRequiresAPIKey(ProviderName, true)
}
