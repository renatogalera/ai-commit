package deepseek

import (
	"context"

	"github.com/renatogalera/ai-commit/pkg/ai"
	"github.com/renatogalera/ai-commit/pkg/config"
	"github.com/renatogalera/ai-commit/pkg/provider/registry"
)

const ProviderName = "deepseek"

func factory(ctx context.Context, name string, ps config.ProviderSettings) (ai.AIClient, error) {
    return NewDeepseekClient(name, ps.APIKey, ps.Model, ps.BaseURL)
}

func init() {
    registry.Register(ProviderName, factory)
    registry.RegisterDefaults(ProviderName, config.ProviderSettings{Model: "deepseek-chat", BaseURL: "https://api.deepseek.com/v1"})
    registry.SetRequiresAPIKey(ProviderName, true)
}
