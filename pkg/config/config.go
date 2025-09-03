package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

const (
    DefaultProvider         = "phind"
)

var (
	DefaultAuthorName  = "ai-commit"
	DefaultAuthorEmail = "ai-commit@example.com"
)

type CommitTypeConfig struct {
    Type  string `yaml:"type,omitempty"`
    Emoji string `yaml:"emoji,omitempty"`
}

// ProviderSettings holds credentials and routing for a provider.
type ProviderSettings struct {
    APIKey  string `yaml:"apiKey,omitempty"`
    Model   string `yaml:"model,omitempty"`
    BaseURL string `yaml:"baseURL,omitempty"`
}

type LimitSettings struct {
    Enabled  bool `yaml:"enabled,omitempty"`
    MaxChars int  `yaml:"maxChars,omitempty"`
}

type Limits struct {
    Diff   LimitSettings `yaml:"diff,omitempty"`
    Prompt LimitSettings `yaml:"prompt,omitempty"`
}

type Config struct {
	Prompt           string             `yaml:"prompt,omitempty"`
	CommitType       string             `yaml:"commitType,omitempty"`
	Template         string             `yaml:"template,omitempty"`
	SemanticRelease  bool               `yaml:"semanticRelease,omitempty"`
	InteractiveSplit bool               `yaml:"interactiveSplit,omitempty"`
	EnableEmoji      bool               `yaml:"enableEmoji,omitempty"`

    Provider    string             `yaml:"provider,omitempty"`
    CommitTypes []CommitTypeConfig `yaml:"commitTypes,omitempty"`
    LockFiles   []string           `yaml:"lockFiles,omitempty"`
    Limits Limits `yaml:"limits,omitempty"`

    // Enterprise-style provider configuration. Preferred over legacy flat fields below.
    Providers map[string]ProviderSettings `yaml:"providers,omitempty"`

    PromptTemplate string `yaml:"promptTemplate,omitempty"`

	AuthorName  string `yaml:"authorName,omitempty"`
	AuthorEmail string `yaml:"authorEmail,omitempty"`
}

func LoadOrCreateConfig() (*Config, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to determine executable path: %w", err)
	}
	binaryName := filepath.Base(exePath)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to determine user home directory: %w", err)
	}
	configDir := filepath.Join(homeDir, ".config", binaryName)
	configPath := filepath.Join(configDir, "config.yaml")

	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create config directory: %w", err)
		}
	}

    if _, err := os.Stat(configPath); os.IsNotExist(err) {
        defaultCfg := &Config{
            Provider:      DefaultProvider,
            AuthorName:    DefaultAuthorName,
            AuthorEmail:   DefaultAuthorEmail,
            LockFiles:     []string{"go.mod", "go.sum"},
            Limits: Limits{
                Diff:   LimitSettings{Enabled: false, MaxChars: 0},
                Prompt: LimitSettings{Enabled: false, MaxChars: 0},
            },
            CommitTypes: []CommitTypeConfig{
                {Type: "feat", Emoji: "✨"},
                {Type: "fix", Emoji: "🐛"},
                {Type: "docs", Emoji: "📚"},
                {Type: "style", Emoji: "💎"},
                {Type: "refactor", Emoji: "♻️"},
                {Type: "test", Emoji: "🧪"},
                {Type: "chore", Emoji: "🔧"},
                {Type: "perf", Emoji: "🚀"},
                {Type: "build", Emoji: "📦"},
                {Type: "ci", Emoji: "👷"},
            },
            Providers: map[string]ProviderSettings{},
            PromptTemplate: "",
        }
        if err := saveConfig(configPath, defaultCfg); err != nil {
            return nil, fmt.Errorf("failed to create default config: %w", err)
        }
        return defaultCfg, nil
    }

    data, err := os.ReadFile(configPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read config file: %w", err)
    }
    var cfg Config
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("failed to parse config file: %w", err)
    }
    return &cfg, nil
}

func saveConfig(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

func ResolveAPIKey(flagVal, envVar, configVal, provider string) (string, error) {
	if strings.TrimSpace(flagVal) != "" {
		return strings.TrimSpace(flagVal), nil
	}
	if envVal := os.Getenv(envVar); strings.TrimSpace(envVal) != "" {
		return strings.TrimSpace(envVal), nil
	}
	if strings.TrimSpace(configVal) != "" {
		return strings.TrimSpace(configVal), nil
	}
 
	return "", fmt.Errorf("%s API key is required. Provide via flag, %s environment variable, or config", provider, envVar)
}

func (cfg *Config) Validate() error {
    v := validator.New()
    if err := v.Struct(cfg); err != nil {
        return fmt.Errorf("config validation failed: %w", err)
    }
    return nil
}

// GetProviderSettings fetches settings from the Providers map and fills defaults.
func (cfg *Config) GetProviderSettings(name string) ProviderSettings {
    if cfg.Providers != nil {
        if ps, ok := cfg.Providers[name]; ok {
            return ps
        }
    }
    return ProviderSettings{}
}
