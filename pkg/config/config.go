package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds the configuration for AI‑Commit.
type Config struct {
	Prompt           string `yaml:"prompt,omitempty"`
	CommitType       string `yaml:"commitType,omitempty"`
	Template         string `yaml:"template,omitempty"`
	SemanticRelease  bool   `yaml:"semanticRelease,omitempty"`
	InteractiveSplit bool   `yaml:"interactiveSplit,omitempty"`
	EnableEmoji      bool   `yaml:"enableEmoji,omitempty"`
	Provider         string `yaml:"provider,omitempty"`

	OpenAIAPIKey    string `yaml:"openAiApiKey,omitempty"`
	OpenAIModel     string `yaml:"openaiModel,omitempty"`
	GeminiAPIKey    string `yaml:"geminiApiKey,omitempty"`
	GeminiModel     string `yaml:"geminiModel,omitempty"`
	AnthropicAPIKey string `yaml:"anthropicApiKey,omitempty"`
	AnthropicModel  string `yaml:"anthropicModel,omitempty"`
	DeepseekAPIKey  string `yaml:"deepseekApiKey,omitempty"`
	DeepseekModel   string `yaml:"deepseekModel,omitempty"`

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
			Provider:         "openai",
			OpenAIAPIKey:     "",
			OpenAIModel:      "chatgpt-4o-latest",
			GeminiAPIKey:     "",
			GeminiModel:      "models/gemini-2.0-flash",
			AnthropicAPIKey:  "",
			AnthropicModel:   "claude-3-5-sonnet-latest",
			DeepseekAPIKey:   "",
			DeepseekModel:    "deepseek-chat",
			AuthorName:       "ai-commit",
			AuthorEmail:      "ai-commit@example.com",
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

