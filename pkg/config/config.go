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
	DefaultOpenAIModel      = "chatgpt-4o-latest"
	DefaultGoogleModel      = "models/gemini-2.5-flash"
	DefaultAnthropicModel   = "claude-3-7-sonnet-latest"
	DefaultDeepseekModel    = "deepseek-chat"
	DefaultPhindModel       = "Phind-70B"
	DefaultOllamaModel      = "llama2"
	DefaultOpenAIBaseURL    = "https://api.openai.com/v1"
	DefaultGoogleBaseURL    = "https://generativelanguage.googleapis.com"
	DefaultAnthropicBaseURL = "https://api.anthropic.com/v1"
	DefaultDeepseekBaseURL  = "https://api.deepseek.com/v1"
	DefaultPhindBaseURL     = "https://https.extension.phind.com/agent/"
	DefaultOllamaBaseURL    = "http://localhost:11434"
)

var (
	DefaultAuthorName  = "ai-commit"
	DefaultAuthorEmail = "ai-commit@example.com"
)

// CommitTypeConfig holds a commit type + its optional emoji.
// This is loaded from config.yaml so we can easily add/delete as needed.
type CommitTypeConfig struct {
	Type  string `yaml:"type,omitempty"`
	Emoji string `yaml:"emoji,omitempty"`
}

type Config struct {
	Prompt           string             `yaml:"prompt,omitempty"`
	CommitType       string             `yaml:"commitType,omitempty"`
	Template         string             `yaml:"template,omitempty"`
	SemanticRelease  bool               `yaml:"semanticRelease,omitempty"`
	InteractiveSplit bool               `yaml:"interactiveSplit,omitempty"`
	EnableEmoji      bool               `yaml:"enableEmoji,omitempty"`
	Provider         string             `yaml:"provider,omitempty" validate:"omitempty,oneof=openai google anthropic deepseek phind ollama"`
	CommitTypes      []CommitTypeConfig `yaml:"commitTypes,omitempty"`
	LockFiles        []string           `yaml:"lockFiles,omitempty"`

	OpenAIAPIKey     string `yaml:"openAiApiKey,omitempty"`
	OpenAIModel      string `yaml:"openaiModel,omitempty"`
	OpenAIBaseURL    string `yaml:"openaiBaseURL,omitempty"`
	GoogleAPIKey     string `yaml:"googleApiKey,omitempty"`
	GoogleModel      string `yaml:"googleModel,omitempty"`
	GoogleBaseURL    string `yaml:"googleBaseURL,omitempty"`
	AnthropicAPIKey  string `yaml:"anthropicApiKey,omitempty"`
	AnthropicModel   string `yaml:"anthropicModel,omitempty"`
	AnthropicBaseURL string `yaml:"anthropicBaseURL,omitempty"`
	DeepseekAPIKey   string `yaml:"deepseekApiKey,omitempty"`
	DeepseekModel    string `yaml:"deepseekModel,omitempty"`
	DeepseekBaseURL  string `yaml:"deepseekBaseURL,omitempty"`
	PhindAPIKey      string `yaml:"phindApiKey,omitempty"`
	PhindModel       string `yaml:"phindModel,omitempty"`
	PhindBaseURL     string `yaml:"phindBaseURL,omitempty"`
	OllamaBaseURL    string `yaml:"ollamaBaseURL,omitempty"`
	OllamaModel      string `yaml:"ollamaModel,omitempty"`
	PromptTemplate   string `yaml:"promptTemplate,omitempty"`

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
			Provider:         DefaultProvider,
			OpenAIAPIKey:     "",
			OpenAIModel:      DefaultOpenAIModel,
			OpenAIBaseURL:    DefaultOpenAIBaseURL,
			GoogleAPIKey:     "",
			GoogleModel:      DefaultGoogleModel,
			GoogleBaseURL:    DefaultGoogleBaseURL,
			AnthropicAPIKey:  "",
			AnthropicModel:   DefaultAnthropicModel,
			AnthropicBaseURL: DefaultAnthropicBaseURL,
			DeepseekAPIKey:   "",
			DeepseekModel:    DefaultDeepseekModel,
			DeepseekBaseURL:  DefaultDeepseekBaseURL,
			PhindAPIKey:      "",
			PhindModel:       DefaultPhindModel,
			PhindBaseURL:     DefaultPhindBaseURL,
			OllamaBaseURL:    DefaultOllamaBaseURL,
			OllamaModel:      DefaultOllamaModel,
			AuthorName:       DefaultAuthorName,
			AuthorEmail:      DefaultAuthorEmail,
			LockFiles:        []string{"go.mod", "go.sum"},
			// Default commit types and emojis:
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
	return os.WriteFile(path, data, 0644)
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
	// For providers like phind that can accept an empty API key, do not return error if empty.
	if provider == "phind" {
		return "", nil
	}
	return "", fmt.Errorf("%s API key is required. Provide via flag, %s environment variable, or config", provider, envVar)
}

func (cfg *Config) Validate() error {
	v := validator.New()
	err := v.Struct(cfg)
	if err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}
	return nil
}
