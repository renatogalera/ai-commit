package deepseek

import (
    "fmt"
    "strings"

    openaic "github.com/renatogalera/ai-commit/pkg/provider/openai_compat"
)

// NewDeepseekClient returns a client using the OpenAI-compatible SDK against DeepSeek's endpoint.
// BaseURL e model devem ser providos pelo registro/config; não definimos fallback aqui.
func NewDeepseekClient(provider, apiKey, model, baseURL string) (*openaic.Client, error) {
    if strings.TrimSpace(baseURL) == "" {
        return nil, fmt.Errorf("deepseek baseURL is required")
    }
    if strings.TrimSpace(model) == "" {
        return nil, fmt.Errorf("deepseek model is required")
    }
    return openaic.NewCompatClient(provider, apiKey, model, baseURL), nil
}
