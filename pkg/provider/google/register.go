package google

import (
	"context"

	"github.com/renatogalera/ai-commit/pkg/ai"
	"github.com/renatogalera/ai-commit/pkg/config"
	"github.com/renatogalera/ai-commit/pkg/provider/registry"
)

const ProviderName = "google"

func factory(ctx context.Context, name string, ps config.ProviderSettings) (ai.AIClient, error) {
    gm, err := NewGoogleProClient(ctx, ps.APIKey, ps.Model, ps.BaseURL)
    if err != nil {
        return nil, err
    }
    return NewClient(name, gm), nil
}

func init() {
    registry.Register(ProviderName, factory)
    registry.RegisterDefaults(ProviderName, config.ProviderSettings{Model: "models/gemini-2.5-flash", BaseURL: "https://generativelanguage.googleapis.com"})
    registry.SetRequiresAPIKey(ProviderName, true)
}
