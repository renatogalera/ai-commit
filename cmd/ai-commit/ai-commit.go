package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/renatogalera/ai-commit/pkg/ai"
	"github.com/renatogalera/ai-commit/pkg/committypes"
	"github.com/renatogalera/ai-commit/pkg/config"
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

var (
	apiKeyFlag           string
	geminiAPIKeyFlag     string
	anthropicAPIKeyFlag  string
	deepseekAPIKeyFlag   string
	languageFlag         string
	commitTypeFlag       string
	templateFlag         string
	forceFlag            bool
	semanticReleaseFlag  bool
	interactiveSplitFlag bool
	emojiFlag            bool
	manualSemverFlag     bool
	providerFlag         string
	modelFlag            string
)

var rootCmd = &cobra.Command{
	Use:   "ai-commit",
	Short: "AI-Commit: Generate Git commit messages with AI",
	Long: `AI-Commit is a CLI tool that generates commit messages using AI providers like OpenAI, Gemini, Anthropic, and Deepseek.
It helps you write better commit messages following the Conventional Commits standard.`,
	Run: runAiCommit,
}

func init() {
	rootCmd.Flags().StringVar(&apiKeyFlag, "apiKey", "", "API key for OpenAI provider (or env OPENAI_API_KEY)")
	rootCmd.Flags().StringVar(&geminiAPIKeyFlag, "geminiApiKey", "", "API key for Gemini provider (or env GEMINI_API_KEY)")
	rootCmd.Flags().StringVar(&anthropicAPIKeyFlag, "anthropicApiKey", "", "API key for Anthropic provider (or env ANTHROPIC_API_KEY)")
	rootCmd.Flags().StringVar(&deepseekAPIKeyFlag, "deepseekApiKey", "", "API key for Deepseek provider (or env DEEPSEEK_API_KEY)")
	rootCmd.Flags().StringVar(&languageFlag, "language", "english", "Language for commit message")
	rootCmd.Flags().StringVar(&commitTypeFlag, "commit-type", "", "Commit type (e.g., feat, fix)")
	rootCmd.Flags().StringVar(&templateFlag, "template", "", "Commit message template")
	rootCmd.Flags().BoolVar(&forceFlag, "force", false, "Bypass interactive UI and commit directly")
	rootCmd.Flags().BoolVar(&semanticReleaseFlag, "semantic-release", false, "Perform semantic release")
	rootCmd.Flags().BoolVar(&interactiveSplitFlag, "interactive-split", false, "Launch interactive commit splitting")
	rootCmd.Flags().BoolVar(&emojiFlag, "emoji", false, "Include emoji in commit message")
	rootCmd.Flags().BoolVar(&manualSemverFlag, "manual-semver", false, "Manually select semantic version bump")
	rootCmd.Flags().StringVar(&providerFlag, "provider", "", "AI provider: openai, gemini, anthropic, deepseek")
	rootCmd.Flags().StringVar(&modelFlag, "model", "", "Sub-model for the chosen provider")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runAiCommit(cmd *cobra.Command, args []string) {
	setupLogger()

	cfg, err := loadConfiguration()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
		os.Exit(1)
	}

	git.CommitAuthorName = cfg.AuthorName
	git.CommitAuthorEmail = cfg.AuthorEmail

	if err := validateFlagsAndConfig(cfg); err != nil {
		log.Error().Err(err).Msg("Configuration validation failed")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	aiClient, err := initAIClient(ctx, cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize AI client")
		os.Exit(1)
	}

	if !git.IsGitRepository(ctx) {
		log.Fatal().Msg("Not a valid Git repository")
		os.Exit(1)
	}

	if interactiveSplitFlag {
		runInteractiveSplit(ctx, aiClient, semanticReleaseFlag, manualSemverFlag)
		return
	}

	diff, err := git.GetGitDiff(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get Git diff")
		os.Exit(1)
	}

	diff = git.FilterLockFiles(diff, cfg.LockFiles)
	if strings.TrimSpace(diff) == "" {
		fmt.Println("No staged changes found after filtering lock files.")
		os.Exit(0)
	}

	promptText := prompt.BuildPrompt(diff, languageFlag, commitTypeFlag, "", cfg.PromptTemplate)

	commitMsg, err := generateCommitMessage(ctx, aiClient, promptText, commitTypeFlag, templateFlag, emojiFlag)
	if err != nil {
		log.Error().Err(err).Msg("Commit message generation failed")
		os.Exit(1)
	}

	if forceFlag {
		handleForceCommit(ctx, commitMsg, aiClient, manualSemverFlag)
		return
	}

	runInteractiveUI(ctx, commitMsg, diff, promptText, aiClient)
}

func setupLogger() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
}

func loadConfiguration() (*config.Config, error) {
	// [Lógica de carregamento do config, sem alterações]
	return config.LoadOrCreateConfig()
}

func validateFlagsAndConfig(cfg *config.Config) error {
	// [Validação de flags e config, sem alterações]
	return cfg.Validate()
}

func initAIClient(ctx context.Context, cfg *config.Config) (ai.AIClient, error) {
	return initProviderClient(ctx, cfg.Provider, cfg)
}

func initProviderClient(ctx context.Context, provider string, cfg *config.Config) (ai.AIClient, error) {
	switch provider {
	case "openai":
		key, err := config.ResolveAPIKey(cfg.OpenAIAPIKey, "OPENAI_API_KEY", cfg.OpenAIAPIKey, "openai")
		if err != nil {
			return nil, err
		}
		return openai.NewOpenAIClient(key, cfg.OpenAIModel), nil

	case "gemini":
		key, err := config.ResolveAPIKey(cfg.GeminiAPIKey, "GEMINI_API_KEY", cfg.GeminiAPIKey, "gemini")
		if err != nil {
			return nil, err
		}
		geminiClient, err := gemini.NewGeminiProClient(ctx, key, cfg.GeminiModel)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Gemini client: %w", err)
		}
		return gemini.NewClient(geminiClient), nil

	case "anthropic":
		key, err := config.ResolveAPIKey(cfg.AnthropicAPIKey, "ANTHROPIC_API_KEY", cfg.AnthropicAPIKey, "anthropic")
		if err != nil {
			return nil, err
		}
		anthroClient, err := anthropic.NewAnthropicClient(key, cfg.AnthropicModel)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Anthropic client: %w", err)
		}
		return anthroClient, nil

	case "deepseek":
		key, err := config.ResolveAPIKey(cfg.DeepseekAPIKey, "DEEPSEEK_API_KEY", cfg.DeepseekAPIKey, "deepseek")
		if err != nil {
			return nil, err
		}
		deepseekClient, err := deepseek.NewDeepseekClient(key, cfg.DeepseekModel)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Deepseek client: %w", err)
		}
		return deepseekClient, nil

	default:
		return nil, fmt.Errorf("invalid provider specified: %s", provider)
	}
}

func runInteractiveSplit(ctx context.Context, aiClient ai.AIClient, semanticReleaseFlag, manualSemverFlag bool) {
	if err := splitter.RunInteractiveSplit(ctx, aiClient); err != nil {
		log.Error().Err(err).Msg("Interactive split failed")
		os.Exit(1)
	}
	if semanticReleaseFlag {
		headMsg, _ := git.GetHeadCommitMessage(ctx)
		if err := versioner.PerformSemanticRelease(ctx, aiClient, headMsg, manualSemverFlag); err != nil {
			log.Error().Err(err).Msg("Semantic release failed")
			os.Exit(1)
		}
	}
	os.Exit(0)
}

func handleForceCommit(ctx context.Context, commitMsg string, aiClient ai.AIClient, manualSemverFlag bool) {
	if strings.TrimSpace(commitMsg) == "" {
		log.Error().Msg("Generated commit message is empty; aborting commit.")
		os.Exit(1)
	}
	if err := git.CommitChanges(ctx, commitMsg); err != nil {
		log.Error().Err(err).Msg("Commit failed")
		os.Exit(1)
	}
	fmt.Println("Commit created successfully (forced).")
	if semanticReleaseFlag {
		if err := versioner.PerformSemanticRelease(ctx, aiClient, commitMsg, manualSemverFlag); err != nil {
			log.Error().Err(err).Msg("Semantic release failed")
			os.Exit(1)
		}
	}
	os.Exit(0)
}

func runInteractiveUI(ctx context.Context, commitMsg string, diff string, promptText string, aiClient ai.AIClient) {
	uiModel := ui.NewUIModel(commitMsg, diff, languageFlag, promptText, commitTypeFlag, templateFlag, emojiFlag, aiClient)
	program := ui.NewProgram(uiModel)
	if _, err := program.Run(); err != nil {
		log.Error().Err(err).Msg("UI encountered an error")
		os.Exit(1)
	}

	if semanticReleaseFlag {
		if err := versioner.PerformSemanticRelease(ctx, uiModel.GetAIClient(), uiModel.GetCommitMsg(), manualSemverFlag); err != nil {
			log.Error().Err(err).Msg("Semantic release failed")
			os.Exit(1)
		}
	}
}

// generateCommitMessage handles AI commit message generation and post-processing.
func generateCommitMessage(ctx context.Context, client ai.AIClient, promptText, commitType, tmpl string, enableEmoji bool) (string, error) {
	msg, err := client.GetCommitMessage(ctx, promptText)
	if err != nil {
		return "", err
	}
	// If commitType was not provided, try to infer it from the generated message.
	if commitType == "" {
		commitType = committypes.GuessCommitType(msg)
	}
	msg = client.SanitizeResponse(msg, commitType) // Corrected SanitizeResponse call (method on client)
	// Always prepend commit type if available.
	if commitType != "" {
		msg = git.PrependCommitType(msg, commitType, enableEmoji)
	}
	if tmpl != "" {
		msg, err = template.ApplyTemplate(tmpl, msg)
		if err != nil {
			return "", err
		}
	}
	return strings.TrimSpace(msg), nil
}
