package phind

import (
	"context"

	"github.com/renatogalera/ai-commit/pkg/ai"
	"github.com/renatogalera/ai-commit/pkg/config"
	"github.com/renatogalera/ai-commit/pkg/provider/registry"
)

const ProviderName = "phind"

func factory(ctx context.Context, name string, ps config.ProviderSettings) (ai.AIClient, error) {
    return NewPhindClient(name, ps.APIKey, ps.Model, ps.BaseURL)
}

func init() {
    registry.Register(ProviderName, factory)
    registry.RegisterDefaults(ProviderName, config.ProviderSettings{Model: "Phind-70B", BaseURL: "https://https.extension.phind.com/agent/"})
    registry.SetRequiresAPIKey(ProviderName, false)
}
