package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"

	"github.com/renatogalera/ai-commit/pkg/committypes"
)

// Default values for configuration
const (
	DefaultProvider       = "openai"
	DefaultOpenAIModel    = "chatgpt-4o-latest"
	DefaultGeminiModel    = "models/gemini-2.0-flash"
	DefaultAnthropicModel = "claude-3-5-sonnet-latest"
	DefaultDeepseekModel  = "deepseek-chat"
)

var (
	DefaultAuthorName  = "ai-commit"
	DefaultAuthorEmail = "ai-commit@example.com"
)

// Config holds the configuration for AI‑Commit.
type Config struct {
	Prompt           string   `yaml:"prompt,omitempty"`
	CommitType       string   `yaml:"commitType,omitempty"`
	Template         string   `yaml:"template,omitempty"`
	SemanticRelease  bool     `yaml:"semanticRelease,omitempty"`
	InteractiveSplit bool     `yaml:"interactiveSplit,omitempty"`
	EnableEmoji      bool     `yaml:"enableEmoji,omitempty"`
	Provider         string   `yaml:"provider,omitempty" validate:"omitempty,oneof=openai gemini anthropic deepseek"`
	CommitTypes      []string `yaml:"commitTypes,omitempty"` // Custom commit types
	LockFiles        []string `yaml:"lockFiles,omitempty"`   // Lock files to filter

	OpenAIAPIKey    string `yaml:"openAiApiKey,omitempty"`
	OpenAIModel     string `yaml:"openaiModel,omitempty"`
	GeminiAPIKey    string `yaml:"geminiApiKey,omitempty"`
	GeminiModel     string `yaml:"geminiModel,omitempty"`
	AnthropicAPIKey string `yaml:"anthropicApiKey,omitempty"`
	AnthropicModel  string `yaml:"anthropicModel,omitempty"`
	DeepseekAPIKey  string `yaml:"deepseekApiKey,omitempty"`
	DeepseekModel   string `yaml:"deepseekModel,omitempty"`
	PromptTemplate  string `yaml:"promptTemplate,omitempty"` // Configurable prompt template

	AuthorName  string `yaml:"authorName,omitempty"`
	AuthorEmail string `yaml:"authorEmail,omitempty"`
}

// LoadOrCreateConfig reads the config from ~/.config/<binary>/config.yaml or creates a default one.
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

	// Create config directory if it doesn't exist.
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create config directory: %w", err)
		}
	}

	// If config file does not exist, create a default config.
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultCfg := &Config{
			Prompt:           "",
			CommitType:       "",
			Template:         "",
			SemanticRelease:  false,
			InteractiveSplit: false,
			EnableEmoji:      false,
			Provider:         DefaultProvider,
			OpenAIAPIKey:     "",
			OpenAIModel:      DefaultOpenAIModel,
			GeminiAPIKey:     "",
			GeminiModel:      DefaultGeminiModel,
			AnthropicAPIKey:  "",
			AnthropicModel:   DefaultAnthropicModel,
			DeepseekAPIKey:   "",
			DeepseekModel:    DefaultDeepseekModel,
			AuthorName:       DefaultAuthorName,
			AuthorEmail:      DefaultAuthorEmail,
			CommitTypes:      committypes.AllTypes(),       // Default commit types in config
			LockFiles:        []string{"go.mod", "go.sum"}, // Default lock files
			PromptTemplate:   "",                           // Default prompt template empty

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

	// If commitTypes are defined in config, set them in committypes package
	if len(cfg.CommitTypes) > 0 {
		committypes.SetValidCommitTypes(cfg.CommitTypes)
	}

	return &cfg, nil
}

// saveConfig writes the configuration to the specified path.
func saveConfig(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// ResolveAPIKey returns the API key from flag, environment variable, or config.
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
	return "", fmt.Errorf("%s API key is required. Provide via flag, %s environment variable, or config.", provider, envVar)
}

// Validate validates the Config struct using go-playground/validator.
func (cfg *Config) Validate() error {
	v := validator.New()
	err := v.Struct(cfg)
	if err != nil {
		// Use a type assertion to see if the error is a ValidationErrors,
		// which is more informative.
		if validationErrs, ok := err.(validator.ValidationErrors); ok {
			for _, e := range validationErrs {
				// Can translate each error one at a time.
				return fmt.Errorf("config validation failed on field '%s': %w", e.Field(), e)
			}
		}
		return fmt.Errorf("config validation failed: %w", err)
	}
	return nil
}
