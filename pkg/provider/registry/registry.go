package registry

import (
    "context"
    "sync"

    "github.com/renatogalera/ai-commit/pkg/ai"
    "github.com/renatogalera/ai-commit/pkg/config"
)

// Factory constructs an AI client for a provider using the given settings.
type Factory func(ctx context.Context, name string, ps config.ProviderSettings) (ai.AIClient, error)

var (
    mu        sync.RWMutex
    factories = map[string]Factory{}
    defaults  = map[string]config.ProviderSettings{}
    required  = map[string]bool{}
)

// Register adds a provider factory under the given name.
func Register(name string, f Factory) {
    mu.Lock()
    factories[name] = f
    mu.Unlock()
}

// Get returns the factory for name if registered.
func Get(name string) (Factory, bool) {
    mu.RLock()
    f, ok := factories[name]
    mu.RUnlock()
    return f, ok
}

// Has reports whether a provider is registered.
func Has(name string) bool {
    mu.RLock()
    _, ok := factories[name]
    mu.RUnlock()
    return ok
}

// Names returns a snapshot of registered provider names.
func Names() []string {
    mu.RLock()
    out := make([]string, 0, len(factories))
    for k := range factories {
        out = append(out, k)
    }
    mu.RUnlock()
    return out
}

// RegisterDefaults sets the default settings for a provider.
func RegisterDefaults(name string, ps config.ProviderSettings) {
    mu.Lock()
    defaults[name] = ps
    mu.Unlock()
}

// SetRequiresAPIKey marks whether a provider requires an API key.
func SetRequiresAPIKey(name string, req bool) {
    mu.Lock()
    required[name] = req
    mu.Unlock()
}

// GetDefaults returns defaults for a provider if registered.
func GetDefaults(name string) (config.ProviderSettings, bool) {
    mu.RLock()
    d, ok := defaults[name]
    mu.RUnlock()
    return d, ok
}

// RequiresAPIKey reports whether the provider requires an API key.
func RequiresAPIKey(name string) bool {
    mu.RLock()
    r := required[name]
    mu.RUnlock()
    return r
}
