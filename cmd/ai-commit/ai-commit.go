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

const (
	defaultTimeout = 60 * time.Second

	// Common error messages as constants for consistency
	errMsgLoadConfig       = "Failed to load configuration"
	errMsgNotGitRepository = "Not a valid Git repository"
	errMsgInitAIClient     = "Failed to initialize AI client"
	errMsgGetDiff          = "Failed to get Git diff"
	errMsgValidation       = "Configuration validation failed"
)

// Global flags. Note that languageFlag is set by a Cobra flag.
var (
	apiKeyFlag          string
	geminiAPIKeyFlag    string
	anthropicAPIKeyFlag string
	deepseekAPIKeyFlag  string

	// languageFlag holds the language for commit messages or reviews.
	// It's set via a Cobra flag: --language, defaulting to "english".
	languageFlag string

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

// rootCmd is the primary Cobra command for ai-commit.
var rootCmd = &cobra.Command{
	Use:   "ai-commit",
	Short: "AI-Commit: Generate Git commit messages and review code with AI",
	Long: `AI-Commit is a CLI tool that generates commit messages and reviews code using AI providers.
It helps you write better commits and get basic AI-powered code reviews.`,
	Run: runAICommit,
}

// reviewCmd is the subcommand for AI-based code review.
var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Review code changes using AI",
	Long:  "Send the current Git diff to AI for a basic code review and get suggestions.",
	Run:   runAICodeReview,
}

func init() {
	// Register flags on the root command
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

	// Add subcommands
	rootCmd.AddCommand(reviewCmd)
}

// main sets up logging once, then executes the root command.
func main() {
	setupLogger()
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// runAICommit is the main function for generating commit messages and committing.
func runAICommit(cmd *cobra.Command, args []string) {
	// Prepare environment: load config, init AI client, ensure valid Git repo
	ctx, cancel, cfg, aiClient, err := setupAIEnvironment()
	if err != nil {
		log.Fatal().Err(err).Msg("setupAIEnvironment error")
	}
	defer cancel()

	// If interactive split is requested, run the interactive splitter TUI
	if interactiveSplitFlag {
		runInteractiveSplit(ctx, aiClient, semanticReleaseFlag, manualSemverFlag)
		return
	}

	// Obtain Git diff
	diff, err := git.GetGitDiff(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg(errMsgGetDiff)
	}

	// Filter lock files from diff
	diff = git.FilterLockFiles(diff, cfg.LockFiles)
	if strings.TrimSpace(diff) == "" {
		fmt.Println("No staged changes found after filtering lock files.")
		return
	}

	// Build prompt text for AI
	promptText := prompt.BuildCommitPrompt(
		diff,
		languageFlag,
		commitTypeFlag,
		"",
		cfg.PromptTemplate,
	)

	// Generate commit message
	commitMsg, err := generateCommitMessage(ctx, aiClient, promptText, commitTypeFlag, templateFlag, emojiFlag)
	if err != nil {
		log.Fatal().Err(err).Msg("Commit message generation failed")
	}

	if forceFlag {
		handleForceCommit(ctx, commitMsg, aiClient)
		return
	}

	// Launch the interactive TUI
	runInteractiveUI(ctx, commitMsg, diff, promptText, aiClient)
}

// runAICodeReview is the subcommand for AI-based code review.
func runAICodeReview(cmd *cobra.Command, args []string) {
	// Prepare environment: load config, init AI client, ensure valid Git repo
	ctx, cancel, cfg, aiClient, err := setupAIEnvironment()
	if err != nil {
		log.Fatal().Err(err).Msg("setupAIEnvironment error")
	}
	defer cancel()

	// Obtain Git diff
	diff, err := git.GetGitDiff(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get Git diff for review")
	}

	if strings.TrimSpace(diff) == "" {
		fmt.Println("No staged changes found for code review.")
		return
	}

	// Build and send the code review prompt
	reviewPrompt := prompt.BuildCodeReviewPrompt(diff, languageFlag, cfg.PromptTemplate)
	reviewResult, err := aiClient.GetCommitMessage(ctx, reviewPrompt)
	if err != nil {
		log.Fatal().Err(err).Msg("Code review generation failed")
	}

	fmt.Println("\nAI Code Review Suggestions:")
	fmt.Println(strings.TrimSpace(reviewResult))
}

// setupLogger configures zerolog to output to console in a user-friendly format.
func setupLogger() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
}

// setupAIEnvironment centralizes the repeated steps of loading config, validating flags,
// initializing the AI client, and ensuring the current directory is a Git repository.
func setupAIEnvironment() (context.Context, context.CancelFunc, *config.Config, ai.AIClient, error) {
	// Load config
	cfg, err := config.LoadOrCreateConfig()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("%s: %w", errMsgLoadConfig, err)
	}

	// Validate config & flags (returns a copy of cfg if needed)
	cfgCopy, err := validateFlagsAndConfig(cfg)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("%s: %w", errMsgValidation, err)
	}

	// Initialize a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)

	// Initialize AI client
	aiClient, err := initAIClient(ctx, cfgCopy)
	if err != nil {
		cancel()
		return nil, nil, nil, nil, fmt.Errorf("%s: %w", errMsgInitAIClient, err)
	}

	// Check if we're in a valid Git repo
	if !git.IsGitRepository(ctx) {
		cancel()
		return nil, nil, nil, nil, fmt.Errorf(errMsgNotGitRepository)
	}

	// Set commit author details
	git.CommitAuthorName = cfgCopy.AuthorName
	git.CommitAuthorEmail = cfgCopy.AuthorEmail

	return ctx, cancel, cfgCopy, aiClient, nil
}

// validateFlagsAndConfig returns a copy of the config with provider set (if empty),
// checks if it's valid, and verifies that the commit type is valid if provided.
func validateFlagsAndConfig(c *config.Config) (*config.Config, error) {
	// Make a local copy to avoid mutating original config
	cfgCopy := *c

	// If providerFlag was specified via CLI, override config
	if providerFlag != "" {
		cfgCopy.Provider = providerFlag
	}
	if cfgCopy.Provider == "" {
		cfgCopy.Provider = config.DefaultProvider
	}

	if !isValidProvider(cfgCopy.Provider) {
		return nil, fmt.Errorf("invalid provider: %s", cfgCopy.Provider)
	}

	// If user provided a commitTypeFlag, check it's valid
	if commitTypeFlag != "" && !committypes.IsValidCommitType(commitTypeFlag) {
		return nil, fmt.Errorf("invalid commit type: %s", commitTypeFlag)
	}

	// Validate the final config
	if err := cfgCopy.Validate(); err != nil {
		return nil, err
	}

	return &cfgCopy, nil
}

// isValidProvider checks if the provider is one of the known valid providers.
func isValidProvider(provider string) bool {
	validProviders := map[string]bool{
		"openai":    true,
		"gemini":    true,
		"anthropic": true,
		"deepseek":  true,
	}
	return validProviders[provider]
}

// initAIClient constructs the appropriate AI client based on the selected provider.
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

// generateCommitMessage obtains an AI-based commit message, optionally applies a template,
// ensures it includes the commit type if specified, and handles emoji usage.
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

	// Guess or sanitize commit type
	if commitType == "" {
		commitType = committypes.GuessCommitType(msg)
	}
	msg = client.SanitizeResponse(msg, commitType)

	// Prepend commit type (with or without emoji)
	if commitType != "" {
		msg = git.PrependCommitType(msg, commitType, enableEmoji)
	}

	// Apply custom template if provided
	if tmpl != "" {
		msg, err = template.ApplyTemplate(tmpl, msg)
		if err != nil {
			return "", err
		}
	}
	return strings.TrimSpace(msg), nil
}

// handleForceCommit commits the generated message directly, bypassing the TUI.
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

// runInteractiveUI launches the TUI for commit message confirmation, regeneration, or editing.
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

	// After user commits, if semantic release is on, do it
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

// runInteractiveSplit handles the interactive commit splitting logic.
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
	// Optionally perform semantic release after the final chunk commit
	if semanticReleaseFlag {
		headMsg, _ := git.GetHeadCommitMessage(ctx)
		if err := versioner.PerformSemanticRelease(ctx, aiClient, headMsg, manualSemverFlag); err != nil {
			log.Error().Err(err).Msg("Semantic release failed")
		}
	}
}
