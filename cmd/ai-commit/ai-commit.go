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
	config "github.com/renatogalera/ai-commit/pkg/config"
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

const (
	defaultTimeout = 60 * time.Second

	errMsgLoadConfig       = "Failed to load configuration"
	errMsgNotGitRepository = "Not a valid Git repository"
	errMsgInitAIClient     = "Failed to initialize AI client"
	errMsgGetDiff          = "Failed to get Git diff"
	errMsgValidation       = "Configuration validation failed"
)

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
	reviewMessageFlag    bool // Flag for commit message style review
)

var rootCmd = &cobra.Command{
	Use:   "ai-commit",
	Short: "AI-Commit: Generate Git commit messages and review code with AI",
	Long: `AI-Commit is a CLI tool that generates commit messages and reviews code using AI providers.
It helps you write better commits and get basic AI-powered code reviews.`,
	Run: runAICommit,
}

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Review code changes using AI",
	Long:  "Send the current Git diff to AI for a basic code review and get suggestions.",
	Run:   runAICodeReview,
}

func init() {
	rootCmd.Flags().StringVar(&apiKeyFlag, "apiKey", "", "API key for OpenAI provider (or env OPENAI_API_KEY)")
	rootCmd.Flags().StringVar(&geminiAPIKeyFlag, "geminiApiKey", "", "API key for Gemini provider (or env GEMINI_API_KEY)")
	rootCmd.Flags().StringVar(&anthropicAPIKeyFlag, "anthropicApiKey", "", "API key for Anthropic provider (or env ANTHROPIC_API_KEY)")
	rootCmd.Flags().StringVar(&deepseekAPIKeyFlag, "deepseekApiKey", "", "API key for Deepseek provider (or env DEEPSEEK_API_KEY)")

	rootCmd.Flags().StringVar(&languageFlag, "language", "english", "Language for commit message/review")
	rootCmd.Flags().StringVar(&commitTypeFlag, "commit-type", "", "Commit type (e.g., feat, fix)")
	rootCmd.Flags().StringVar(&templateFlag, "template", "", "Commit message template")
	rootCmd.Flags().BoolVar(&forceFlag, "force", false, "Bypass interactive UI and commit directly")
	rootCmd.Flags().BoolVar(&semanticReleaseFlag, "semantic-release", false, "Perform semantic release")
	rootCmd.Flags().BoolVar(&interactiveSplitFlag, "interactive-split", false, "Launch interactive commit splitting")
	rootCmd.Flags().BoolVar(&emojiFlag, "emoji", false, "Include emoji in commit message")
	rootCmd.Flags().BoolVar(&manualSemverFlag, "manual-semver", false, "Manually select semantic version bump")
	rootCmd.Flags().StringVar(&providerFlag, "provider", "", "AI provider: openai, gemini, anthropic, deepseek")
	rootCmd.Flags().StringVar(&modelFlag, "model", "", "Sub-model for the chosen provider")
	rootCmd.Flags().BoolVar(&reviewMessageFlag, "review-message", false, "Review and enforce commit message style using AI") // Add review-message flag

	rootCmd.AddCommand(reviewCmd)
}

func main() {
	setupLogger()
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runAICommit(cmd *cobra.Command, args []string) {
	ctx, cancel, cfg, aiClient, err := setupAIEnvironment()
	if err != nil {
		log.Fatal().Err(err).Msg("Setup AI environment error")
		return
	}
	defer cancel()

	if interactiveSplitFlag {
		runInteractiveSplit(ctx, aiClient, semanticReleaseFlag, manualSemverFlag)
		return
	}

	diff, err := git.GetGitDiff(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg(errMsgGetDiff)
		return
	}

	diff = git.FilterLockFiles(diff, cfg.LockFiles)
	if strings.TrimSpace(diff) == "" {
		fmt.Println("No staged changes after filtering lock files.")
		return
	}

	promptText := prompt.BuildCommitPrompt(diff, languageFlag, commitTypeFlag, "", cfg.PromptTemplate)
	commitMsg, genErr := generateCommitMessage(ctx, aiClient, promptText, commitTypeFlag, templateFlag, emojiFlag)
	if genErr != nil {
		log.Error().Err(genErr).Msg("Commit message generation error")
		os.Exit(1) // Exit directly when message generation fails in non-interactive mode
	}

	if reviewMessageFlag {
		commitMsg, err = enforceCommitMessageStyle(ctx, aiClient, commitMsg, languageFlag, cfg.PromptTemplate)
		if err != nil {
			log.Error().Err(err).Msg("Commit message style enforcement failed")
			os.Exit(1)
		}
		fmt.Println("\nAI-Reviewed Commit Message:")
		fmt.Println(commitMsg)
	}

	if forceFlag {
		handleForceCommit(ctx, commitMsg, aiClient)
		return
	}

	runInteractiveUI(ctx, commitMsg, diff, promptText, aiClient)
}

func enforceCommitMessageStyle(ctx context.Context, client ai.AIClient, commitMsg string, language string, promptTemplate string) (string, error) {
	reviewPrompt := prompt.BuildCommitStyleReviewPrompt(commitMsg, language, promptTemplate)
	styleReviewResult, err := client.GetCommitMessage(ctx, reviewPrompt)
	if err != nil {
		return commitMsg, fmt.Errorf("commit message style review failed: %w", err)
	}

	if !strings.Contains(strings.ToLower(styleReviewResult), "no issues found") {
		fmt.Println("\nAI Commit Message Style Review Suggestions:")
		fmt.Println(strings.TrimSpace(styleReviewResult))
	} else {
		fmt.Println("\nAI Commit Message Style Review: No issues found. 👍")
	}
	return commitMsg, nil
}

func runAICodeReview(cmd *cobra.Command, args []string) {
	ctx, cancel, cfg, aiClient, err := setupAIEnvironment()
	if err != nil {
		log.Fatal().Err(err).Msg("Setup AI environment error")
		return
	}
	defer cancel()

	diff, err := git.GetGitDiff(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Git diff error")
		return
	}

	if strings.TrimSpace(diff) == "" {
		fmt.Println("No staged changes for code review.")
		return
	}

	reviewPrompt := prompt.BuildCodeReviewPrompt(diff, languageFlag, cfg.PromptTemplate)
	reviewResult, err := aiClient.GetCommitMessage(ctx, reviewPrompt)
	if err != nil {
		log.Fatal().Err(err).Msg("Code review generation error")
		return
	}

	fmt.Println("\nAI Code Review Suggestions:")
	fmt.Println(strings.TrimSpace(reviewResult))
}

// ... (setupLogger, setupAIEnvironment, validateFlagsAndConfig, isValidProvider, initAIClient, initProviderClient, handleForceCommit, runInteractiveUI, runInteractiveSplit, generateCommitMessage - same as before, no changes)
func setupLogger() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
}

func setupAIEnvironment() (context.Context, context.CancelFunc, *config.Config, ai.AIClient, error) {
	cfg, err := config.LoadOrCreateConfig()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("%s: %w", errMsgLoadConfig, err)
	}

	cfgCopy, err := validateFlagsAndConfig(cfg)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("%s: %w", errMsgValidation, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)

	aiClient, err := initAIClient(ctx, cfgCopy)
	if err != nil {
		cancel()
		return nil, nil, nil, nil, fmt.Errorf("%s: %w", errMsgInitAIClient, err)
	}

	if !git.IsGitRepository(ctx) {
		cancel()
		return nil, nil, nil, nil, fmt.Errorf("%s", errMsgNotGitRepository)
	}

	config.DefaultAuthorName = cfgCopy.AuthorName
	config.DefaultAuthorEmail = cfgCopy.AuthorEmail

	return ctx, cancel, cfgCopy, aiClient, nil
}

func validateFlagsAndConfig(cfg *config.Config) (*config.Config, error) {
	cfgCopy := *cfg

	if providerFlag != "" {
		cfgCopy.Provider = providerFlag
	}
	if cfgCopy.Provider == "" {
		cfgCopy.Provider = config.DefaultProvider
	}

	if !isValidProvider(cfgCopy.Provider) {
		return nil, fmt.Errorf("invalid provider: %s", cfgCopy.Provider)
	}

	if commitTypeFlag != "" && !committypes.IsValidCommitType(commitTypeFlag) {
		return nil, fmt.Errorf("invalid commit type: %s", commitTypeFlag)
	}

	if err := cfgCopy.Validate(); err != nil {
		return nil, err
	}

	return &cfgCopy, nil
}

func isValidProvider(provider string) bool {
	validProviders := map[string]bool{
		"openai":    true,
		"gemini":    true,
		"anthropic": true,
		"deepseek":  true,
	}
	return validProviders[provider]
}

func initAIClient(ctx context.Context, cfg *config.Config) (ai.AIClient, error) {
	switch cfg.Provider {
	case "openai":
		key, err := config.ResolveAPIKey(apiKeyFlag, "OPENAI_API_KEY", cfg.OpenAIAPIKey, "openai")
		if err != nil {
			return nil, err
		}
		model := cfg.OpenAIModel
		if modelFlag != "" {
			model = modelFlag
		}
		return openai.NewOpenAIClient(key, model), nil

	case "gemini":
		key, err := config.ResolveAPIKey(geminiAPIKeyFlag, "GEMINI_API_KEY", cfg.GeminiAPIKey, "gemini")
		if err != nil {
			return nil, err
		}
		model := cfg.GeminiModel
		if modelFlag != "" {
			model = modelFlag
		}
		geminiClient, err := gemini.NewGeminiProClient(ctx, key, model)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Gemini client: %w", err)
		}
		return gemini.NewClient(geminiClient), nil

	case "anthropic":
		key, err := config.ResolveAPIKey(anthropicAPIKeyFlag, "ANTHROPIC_API_KEY", cfg.AnthropicAPIKey, "anthropic")
		if err != nil {
			return nil, err
		}
		model := cfg.AnthropicModel
		if modelFlag != "" {
			model = modelFlag
		}
		anthroClient, err := anthropic.NewAnthropicClient(key, model)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Anthropic client: %w", err)
		}
		return anthroClient, nil

	case "deepseek":
		key, err := config.ResolveAPIKey(deepseekAPIKeyFlag, "DEEPSEEK_API_KEY", cfg.DeepseekAPIKey, "deepseek")
		if err != nil {
			return nil, err
		}
		model := cfg.DeepseekModel
		if modelFlag != "" {
			model = modelFlag
		}
		deepseekClient, err := deepseek.NewDeepseekClient(key, model)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Deepseek client: %w", err)
		}
		return deepseekClient, nil
	}

	return nil, fmt.Errorf("invalid provider specified: %s", cfg.Provider)
}

func generateCommitMessage(
	ctx context.Context,
	client ai.AIClient,
	promptText string,
	commitType string,
	tmpl string,
	enableEmoji bool,
) (string, error) {
	msg, err := client.GetCommitMessage(ctx, promptText)
	if err != nil {
		return "", err
	}

	if commitType == "" {
		commitType = committypes.GuessCommitType(msg)
	}
	msg = client.SanitizeResponse(msg, commitType)

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

func handleForceCommit(ctx context.Context, commitMsg string, aiClient ai.AIClient) {
	if strings.TrimSpace(commitMsg) == "" {
		log.Fatal().Msg("Generated commit message is empty; aborting commit.")
	}

	if err := git.CommitChanges(ctx, commitMsg); err != nil {
		log.Fatal().Err(err).Msg("Commit failed")
	}
	fmt.Println("Commit created successfully (forced).")

	if semanticReleaseFlag {
		if err := versioner.PerformSemanticRelease(ctx, aiClient, commitMsg, manualSemverFlag); err != nil {
			log.Fatal().Err(err).Msg("Semantic release failed")
		}
	}
}

func runInteractiveUI(
	ctx context.Context,
	commitMsg string,
	diff string,
	promptText string,
	aiClient ai.AIClient,
) {
	uiModel := ui.NewUIModel(
		commitMsg,
		diff,
		languageFlag,
		promptText,
		commitTypeFlag,
		templateFlag,
		emojiFlag,
		aiClient,
	)
	program := ui.NewProgram(uiModel)
	if _, err := program.Run(); err != nil {
		log.Fatal().Err(err).Msg("UI encountered an error")
	}

	if semanticReleaseFlag {
		if err := versioner.PerformSemanticRelease(
			ctx,
			uiModel.GetAIClient(),
			uiModel.GetCommitMsg(),
			manualSemverFlag,
		); err != nil {
			log.Fatal().Err(err).Msg("Semantic release failed")
		}
	}
}

func runInteractiveSplit(
	ctx context.Context,
	aiClient ai.AIClient,
	semanticReleaseFlag bool,
	manualSemverFlag bool,
) {
	if err := splitter.RunInteractiveSplit(ctx, aiClient); err != nil {
		log.Error().Err(err).Msg("Interactive split failed")
		return
	}
	if semanticReleaseFlag {
		headMsg, _ := git.GetHeadCommitMessage(ctx)
		if err := versioner.PerformSemanticRelease(ctx, aiClient, headMsg, manualSemverFlag); err != nil {
			log.Error().Err(err).Msg("Semantic release failed")
		}
	}
}
