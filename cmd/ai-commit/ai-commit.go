package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"

	gogpt "github.com/sashabaranov/go-openai"

	"github.com/renatogalera/ai-commit/pkg/ai"
	"github.com/renatogalera/ai-commit/pkg/committypes"
	"github.com/renatogalera/ai-commit/pkg/git"
	"github.com/renatogalera/ai-commit/pkg/prompt"
	"github.com/renatogalera/ai-commit/pkg/provider/anthropic"
	"github.com/renatogalera/ai-commit/pkg/provider/deepseek"
	"github.com/renatogalera/ai-commit/pkg/provider/gemini"
	"github.com/renatogalera/ai-commit/pkg/provider/openai"
	"github.com/renatogalera/ai-commit/pkg/template"
	"github.com/renatogalera/ai-commit/pkg/ui"
	"github.com/renatogalera/ai-commit/pkg/ui/splitter"
	"github.com/renatogalera/ai-commit/pkg/versioner"
)

const defaultTimeout = 60 * time.Second

// Add new provider constant near the other provider constants.
const (
	providerOpenAI    = "openai"
	providerGemini    = "gemini"
	providerAnthropic = "anthropic"
	providerDeepseek  = "deepseek"
)

// Update Config struct to include Deepseek-related settings.
type Config struct {
	Prompt           string `yaml:"prompt,omitempty"`
	CommitType       string `yaml:"commitType,omitempty"`
	Template         string `yaml:"template,omitempty"`
	SemanticRelease  bool   `yaml:"semanticRelease,omitempty"`
	InteractiveSplit bool   `yaml:"interactiveSplit,omitempty"`
	EnableEmoji      bool   `yaml:"enableEmoji,omitempty"`
	Provider         string `yaml:"provider,omitempty"`

	// OpenAI-related
	OpenAIAPIKey string `yaml:"openAiApiKey,omitempty"`
	OpenAIModel  string `yaml:"openaiModel,omitempty"`

	// Gemini-related
	GeminiAPIKey string `yaml:"geminiApiKey,omitempty"`
	GeminiModel  string `yaml:"geminiModel,omitempty"`

	// Anthropic-related
	AnthropicAPIKey string `yaml:"anthropicApiKey,omitempty"`
	AnthropicModel  string `yaml:"anthropicModel,omitempty"`

	// Deepseek-related
	DeepseekAPIKey string `yaml:"deepseekApiKey,omitempty"`
	DeepseekModel  string `yaml:"deepseekModel,omitempty"`

	// New commit author configuration
	AuthorName  string `yaml:"authorName,omitempty"`
	AuthorEmail string `yaml:"authorEmail,omitempty"`
}

// LoadOrCreateConfig reads the config from ~/.config/<binary>/config.yaml, or creates it if missing.
func LoadOrCreateConfig() (*Config, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("could not determine executable path: %w", err)
	}
	binaryName := filepath.Base(exePath)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not determine user home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", binaryName)
	configPath := filepath.Join(configDir, "config.yaml")

	// Ensure config directory exists
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		if mkErr := os.MkdirAll(configDir, 0o755); mkErr != nil {
			return nil, fmt.Errorf("failed to create config directory: %w", mkErr)
		}
	}

	// If config.yaml doesn't exist, create with defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultCfg := &Config{
			Prompt:           "",
			CommitType:       "",
			Template:         "",
			SemanticRelease:  false,
			InteractiveSplit: false,
			EnableEmoji:      false,
			Provider:         providerOpenAI,
			OpenAIAPIKey:     "",
			OpenAIModel:      gogpt.GPT4oLatest,
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
			return nil, fmt.Errorf("failed to create default config.yaml: %w", err)
		}
		log.Info().Msgf("No config.yaml found. Created default at %s", configPath)
		return defaultCfg, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config.yaml: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config.yaml: %w", err)
	}
	return &cfg, nil
}

func saveConfig(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}

// loadAPIKey is a small helper that ensures we only set the final API key from
// (1) a CLI flag, (2) an ENV variable, or (3) a config file field.
func loadAPIKey(flagVal, envVarName, configVal, provider string) (string, error) {
	if strings.TrimSpace(flagVal) != "" {
		return strings.TrimSpace(flagVal), nil
	}
	if envVal := os.Getenv(envVarName); strings.TrimSpace(envVal) != "" {
		return strings.TrimSpace(envVal), nil
	}
	if strings.TrimSpace(configVal) != "" {
		return strings.TrimSpace(configVal), nil
	}
	return "", fmt.Errorf("%s API key is required (flag --%sApiKey, env %s, or config.yaml).", provider, provider, envVarName)
}

// initAIClient centralizes all logic for picking which AI provider to use,
// reading the correct key, and instantiating the correct client.
func initAIClient(
	ctx context.Context,
	cfg *Config,
	providerFlag string,
	apiKeyFlag string,
	modelFlag string,
	geminiKeyFlag string,
	anthropicKeyFlag string,
	deepseekKeyFlag string, // New parameter for Deepseek
) (ai.AIClient, error) {

	provider := strings.TrimSpace(providerFlag)
	if provider == "" {
		// fallback from config
		provider = cfg.Provider
	}

	// Validate provider
	if provider != providerOpenAI && provider != providerGemini && provider != providerAnthropic && provider != providerDeepseek {
		return nil, fmt.Errorf("invalid provider specified: %s (must be openai, gemini, anthropic, or deepseek)", provider)
	}

	var finalModel string

	switch provider {
	case providerOpenAI:
		apiKey, err := loadAPIKey(apiKeyFlag, "OPENAI_API_KEY", cfg.OpenAIAPIKey, "openai")
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(modelFlag) != "" {
			finalModel = modelFlag
		} else {
			finalModel = cfg.OpenAIModel
		}
		openAIClient := gogpt.NewClient(apiKey)
		return openai.NewOpenAIClient(openAIClient, finalModel), nil

	case providerGemini:
		apiKey, err := loadAPIKey(geminiKeyFlag, "GEMINI_API_KEY", cfg.GeminiAPIKey, "gemini")
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(modelFlag) != "" {
			finalModel = modelFlag
		} else {
			finalModel = cfg.GeminiModel
		}
		geminiClient, err := gemini.NewGeminiProClient(ctx, apiKey, finalModel)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Gemini client: %w", err)
		}
		return gemini.NewClient(geminiClient), nil

	case providerAnthropic:
		apiKey, err := loadAPIKey(anthropicKeyFlag, "ANTHROPIC_API_KEY", cfg.AnthropicAPIKey, "anthropic")
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(modelFlag) != "" {
			finalModel = modelFlag
		} else {
			finalModel = cfg.AnthropicModel
		}
		anthroClient, err := anthropic.NewAnthropicClient(apiKey, finalModel)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Anthropic client: %w", err)
		}
		return anthroClient, nil

	case providerDeepseek:
		// New case for Deepseek
		apiKey, err := loadAPIKey(deepseekKeyFlag, "DEEPSEEK_API_KEY", cfg.DeepseekAPIKey, "deepseek")
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(modelFlag) != "" {
			finalModel = modelFlag
		} else {
			finalModel = cfg.DeepseekModel
		}
		deepseekClient, err := deepseek.NewDeepseekClient(apiKey, finalModel)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Deepseek client: %w", err)
		}
		return deepseekClient, nil
	}

	return nil, errors.New("no valid AI provider selected")
}

func main() {
	// Initialize logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	// Load or create config
	cfgFile, err := LoadOrCreateConfig()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load config.yaml")
		os.Exit(1)
	}

	// Set commit author details from config to git package global variables
	git.CommitAuthorName = cfgFile.AuthorName
	git.CommitAuthorEmail = cfgFile.AuthorEmail

	// CLI flags
	apiKeyFlag := flag.String("apiKey", "", "API key for the chosen provider (openai). For Gemini or Anthropic see respective flags below.")
	geminiAPIKeyFlag := flag.String("geminiApiKey", cfgFile.GeminiAPIKey, "Gemini API key (or set GEMINI_API_KEY env)")
	anthropicAPIKeyFlag := flag.String("anthropicApiKey", cfgFile.AnthropicAPIKey, "Anthropic API key (or set ANTHROPIC_API_KEY env)")
	deepseekAPIKeyFlag := flag.String("deepseekApiKey", cfgFile.DeepseekAPIKey, "Deepseek API key (or set DEEPSEEK_API_KEY env)")

	languageFlag := flag.String("language", "english", "Language for the commit message")
	commitTypeFlag := flag.String("commit-type", cfgFile.CommitType, "Commit type (e.g. feat, fix, docs)")
	templateFlag := flag.String("template", cfgFile.Template, "Commit message template")
	forceFlag := flag.Bool("force", false, "Automatically commit without TUI")
	semanticReleaseFlag := flag.Bool("semantic-release", cfgFile.SemanticRelease, "Suggest or tag a new version (semantic release)")
	interactiveSplitFlag := flag.Bool("interactive-split", cfgFile.InteractiveSplit, "Interactively split staged changes into multiple commits")
	emojiFlag := flag.Bool("emoji", cfgFile.EnableEmoji, "Include an emoji prefix in commit message")
	manualSemverFlag := flag.Bool("manual-semver", false, "Pick the next version manually instead of using AI suggestion")

	// Instead of --openai-model/--gemini-model/--anthropic-model, use a single --model
	providerFlag := flag.String("provider", cfgFile.Provider, "AI provider to use (openai, gemini, or anthropic)")
	modelFlag := flag.String("model", "", "Sub-model to use for the chosen provider (e.g. gpt-4, models/gemini-2.0, claude-2)")

	flag.Parse()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Initialize the AI client once, with a single function
	aiClient, err := initAIClient(
		ctx,
		cfgFile,
		*providerFlag,
		*apiKeyFlag,
		*modelFlag,
		*geminiAPIKeyFlag,
		*anthropicAPIKeyFlag,
		*deepseekAPIKeyFlag,
	)
	if err != nil {
		log.Error().Err(err).Msg("Unable to initialize AI client")
		os.Exit(1)
	}

	// Verify we're in a Git repository
	if !git.CheckGitRepository(ctx) {
		log.Error().Msg("This is not a Git repository.")
		os.Exit(1)
	}

	// Check commit type validity
	if *commitTypeFlag != "" && !committypes.IsValidCommitType(*commitTypeFlag) {
		log.Error().Msgf("Invalid commit type: %s", *commitTypeFlag)
		os.Exit(1)
	}

	// Interactive split flow (partial commits)
	if *interactiveSplitFlag {
		if err := runInteractiveSplit(ctx, aiClient); err != nil {
			log.Error().Err(err).Msg("Error in interactive split")
			os.Exit(1)
		}
		if *semanticReleaseFlag {
			headMsg, _ := git.GetHeadCommitMessage(ctx)
			if err := doSemanticRelease(ctx, aiClient, headMsg, *manualSemverFlag); err != nil {
				log.Error().Err(err).Msg("Error in semantic release")
				os.Exit(1)
			}
		}
		os.Exit(0)
	}

	// Retrieve diff
	diff, err := git.GetGitDiff(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Error getting Git diff")
		os.Exit(1)
	}
	originalDiff := diff
	diff = git.FilterLockFiles(diff, []string{"go.mod", "go.sum"})
	if len(strings.TrimSpace(diff)) == 0 {
		fmt.Println("No changes to commit (after filtering lock files). Did you stage your changes?")
		os.Exit(0)
	}
	if diff != originalDiff {
		fmt.Println("Note: lock file changes are still committed but not used for AI generation.")
	}

	// Possibly truncate the diff for large changes
	diff, _ = ai.MaybeSummarizeDiff(diff, 5000)

	// Build final prompt
	promptText := prompt.BuildPrompt(diff, *languageFlag, *commitTypeFlag, "")

	// Generate commit message
	commitMsg, err := generateCommitMessage(ctx, aiClient, promptText, *commitTypeFlag, *templateFlag, *emojiFlag)
	if err != nil {
		log.Error().Err(err).Msg("Error generating commit message")
		os.Exit(1)
	}

	// Force commit if requested
	if *forceFlag {
		if strings.TrimSpace(commitMsg) == "" {
			log.Error().Msg("Generated commit message is empty; cannot commit.")
			os.Exit(1)
		}
		if err := git.CommitChanges(ctx, commitMsg); err != nil {
			log.Error().Err(err).Msg("Error committing changes")
			os.Exit(1)
		}
		fmt.Println("Commit created successfully (forced)!")
		if *semanticReleaseFlag {
			if err := doSemanticRelease(ctx, aiClient, commitMsg, *manualSemverFlag); err != nil {
				log.Error().Err(err).Msg("Error in semantic release")
				os.Exit(1)
			}
		}
		os.Exit(0)
	}

	// Otherwise, run interactive TUI
	model := ui.NewUIModel(
		commitMsg,
		diff,
		*languageFlag,
		promptText,
		*commitTypeFlag,
		*templateFlag,
		*emojiFlag,
		aiClient,
	)
	p := ui.NewProgram(model)
	if err := p.Start(); err != nil {
		if errors.Is(err, context.Canceled) {
			os.Exit(0)
		}
		log.Error().Err(err).Msg("TUI error")
		os.Exit(1)
	}

	// If semantic release
	if *semanticReleaseFlag {
		if err := doSemanticRelease(ctx, aiClient, commitMsg, *manualSemverFlag); err != nil {
			log.Error().Err(err).Msg("Error in semantic release")
			os.Exit(1)
		}
	}
}

func generateCommitMessage(
	ctx context.Context,
	client ai.AIClient,
	prompt string,
	commitType string,
	templateStr string,
	enableEmoji bool,
) (string, error) {
	res, err := client.GetCommitMessage(ctx, prompt)
	if err != nil {
		return "", err
	}
	// Clean up
	res = ai.SanitizeResponse(res, commitType)

	// Optionally add Gitmoji
	if enableEmoji {
		res = git.AddGitmoji(res, commitType)
	}

	// Apply template
	if templateStr != "" {
		res, err = template.ApplyTemplate(templateStr, res)
		if err != nil {
			return "", err
		}
	}
	return strings.TrimSpace(res), nil
}

// doSemanticRelease performs the semantic version flow
func doSemanticRelease(ctx context.Context, client ai.AIClient, commitMsg string, manual bool) error {
	log.Info().Msg("Starting semantic release process...")
	currentVersion, err := versioner.GetCurrentVersionTag(ctx)
	if err != nil {
		return fmt.Errorf("could not get current version: %w", err)
	}
	if currentVersion == "" {
		log.Info().Msg("No existing version tag, assuming v0.0.0.")
		currentVersion = "v0.0.0"
	}
	var nextVersion string
	if manual {
		userPicked, err := versioner.RunSemVerTUI(ctx, currentVersion)
		if err != nil {
			return fmt.Errorf("manual semver TUI error: %w", err)
		}
		if userPicked != "" {
			nextVersion = userPicked
			log.Info().Msgf("User selected next version: %s", nextVersion)
		} else {
			log.Info().Msg("User canceled manual semver selection. Skipping semantic release.")
			return nil
		}
	} else {
		aiVer, aiErr := versioner.SuggestNextVersion(ctx, currentVersion, commitMsg, client)
		if aiErr != nil {
			return fmt.Errorf("AI version suggestion error: %w", aiErr)
		}
		nextVersion = aiVer
		log.Info().Msgf("AI-suggested version: %s", nextVersion)
	}
	if err := versioner.CreateLocalTag(ctx, nextVersion); err != nil {
		return fmt.Errorf("failed to create local tag %s: %w", nextVersion, err)
	}
	log.Info().Msgf("Semantic release done! Local tag %s created", nextVersion)
	return nil
}

// runInteractiveSplit handles chunk-based partial commits in a TUI
func runInteractiveSplit(ctx context.Context, client ai.AIClient) error {
	diff, err := git.GetGitDiff(ctx)
	if err != nil {
		return err
	}
	diff = git.FilterLockFiles(diff, []string{"go.mod", "go.sum"})
	if strings.TrimSpace(diff) == "" {
		fmt.Println("No changes to commit (after filtering lock files). Did you stage your changes?")
		return nil
	}
	chunks, err := git.ParseDiffToChunks(diff)
	if err != nil {
		return fmt.Errorf("parseDiffToChunks error: %w", err)
	}
	if len(chunks) == 0 {
		fmt.Println("No diff chunks found.")
		return nil
	}
	model := splitter.NewSplitterModel(chunks, client)
	prog := splitter.NewProgram(model)
	return prog.Start()
}
