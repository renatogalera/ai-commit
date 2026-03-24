package google

import (
	"context"

	"github.com/renatogalera/ai-commit/pkg/ai"
	"github.com/renatogalera/ai-commit/pkg/config"
	"github.com/renatogalera/ai-commit/pkg/provider/registry"
)

const ProviderName = "google"

func factory(ctx context.Context, name string, ps config.ProviderSettings) (ai.AIClient, error) {
	return NewGoogleClient(ctx, name, ps.APIKey, ps.Model, ps.BaseURL)
}

func init() {
	registry.Register(ProviderName, factory)
	registry.RegisterDefaults(ProviderName, config.ProviderSettings{Model: "gemini-2.5-flash", BaseURL: ""})
	registry.SetRequiresAPIKey(ProviderName, true)
}
