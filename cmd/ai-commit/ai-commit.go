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
	"github.com/renatogalera/ai-commit/pkg/provider/phind"
	"github.com/renatogalera/ai-commit/pkg/summarizer"
	"github.com/renatogalera/ai-commit/pkg/template"
	"github.com/renatogalera/ai-commit/pkg/ui"
	"github.com/renatogalera/ai-commit/pkg/ui/splitter"
	"github.com/renatogalera/ai-commit/pkg/versioner"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var (
	apiKeyFlag           string
	geminiAPIKeyFlag     string
	anthropicAPIKeyFlag  string
	deepseekAPIKeyFlag   string
	phindAPIKeyFlag      string
	commitTypeFlag       string
	templateFlag         string
	languageFlag         string
	forceFlag            bool
	semanticReleaseFlag  bool
	interactiveSplitFlag bool
	emojiFlag            bool
	manualSemverFlag     bool
	providerFlag         string
	modelFlag            string
	reviewMessageFlag    bool
)

var rootCmd = &cobra.Command{
	Use:   "ai-commit",
	Short: "AI-Commit: Generate Git commit messages and review code with AI",
	Long:  "AI-Commit is a CLI tool that generates commit messages and reviews code using AI providers.",
}

func init() {
	rootCmd.Run = runAICommit
}

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Review code changes using AI",
	Long:  "Send the current Git diff to AI for a basic code review and get suggestions.",
	Run:   runAICodeReview,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&languageFlag, "language", "english", "Language for commit message/review")

	rootCmd.Flags().StringVar(&apiKeyFlag, "apiKey", "", "API key for OpenAI provider (or env OPENAI_API_KEY)")
	rootCmd.Flags().StringVar(&geminiAPIKeyFlag, "geminiApiKey", "", "API key for Gemini provider (or env GEMINI_API_KEY)")
	rootCmd.Flags().StringVar(&anthropicAPIKeyFlag, "anthropicApiKey", "", "API key for Anthropic provider (or env ANTHROPIC_API_KEY)")
	rootCmd.Flags().StringVar(&deepseekAPIKeyFlag, "deepseekApiKey", "", "API key for Deepseek provider (or env DEEPSEEK_API_KEY)")
	rootCmd.Flags().StringVar(&phindAPIKeyFlag, "phindApiKey", "", "API key for Phind provider (or env PHIND_API_KEY)")

	rootCmd.Flags().StringVar(&commitTypeFlag, "commit-type", "", "Commit type (e.g., feat, fix)")
	rootCmd.Flags().StringVar(&templateFlag, "template", "", "Commit message template")
	rootCmd.Flags().BoolVar(&forceFlag, "force", false, "Bypass interactive UI and commit directly")
	rootCmd.Flags().BoolVar(&semanticReleaseFlag, "semantic-release", false, "Perform semantic release")
	rootCmd.Flags().BoolVar(&interactiveSplitFlag, "interactive-split", false, "Launch interactive commit splitting")
	rootCmd.Flags().BoolVar(&emojiFlag, "emoji", false, "Include emoji in commit message")
	rootCmd.Flags().BoolVar(&manualSemverFlag, "manual-semver", false, "Manually select semantic version bump")
	rootCmd.Flags().StringVar(&providerFlag, "provider", "", "AI provider: openai, gemini, anthropic, deepseek, phind")
	rootCmd.Flags().StringVar(&modelFlag, "model", "", "Sub-model for the chosen provider")
	rootCmd.Flags().BoolVar(&reviewMessageFlag, "review-message", false, "Review and enforce commit message style using AI")

	rootCmd.AddCommand(newSummarizeCmd(setupAIEnvironment))
	rootCmd.AddCommand(reviewCmd)
}

func main() {
	if len(os.Args) > 1 {
		arg := os.Args[1]
		if arg == "--version" || arg == "-v" {
			fmt.Printf("ai-commit version: %s\ncommit: %s\nbuilt at: %s\n", version, commit, date)
			return
		}
	}

	setupLogger()

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func setupLogger() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
}

func setupAIEnvironment() (context.Context, context.CancelFunc, *config.Config, ai.AIClient, error) {
	cfg, err := config.LoadOrCreateConfig()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to load config: %w", err)
	}
	cm := config.NewConfigManager(cfg)
	mergedCfg := cm.MergeConfiguration()

	if mergedCfg.Provider == "" {
		mergedCfg.Provider = config.DefaultProvider
	}
	if !isValidProvider(mergedCfg.Provider) {
		return nil, nil, nil, nil, fmt.Errorf("invalid provider: %s", mergedCfg.Provider)
	}
	if err := mergedCfg.Validate(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("config validation failed: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	committypes.InitCommitTypes(mergedCfg.CommitTypes)

	aiClient, err := initAIClient(ctx, mergedCfg)
	if err != nil {
		cancel()
		return nil, nil, nil, nil, fmt.Errorf("failed to initialize AI client: %w", err)
	}

	if !git.IsGitRepository(ctx) {
		cancel()
		return nil, nil, nil, nil, fmt.Errorf("not a valid Git repository")
	}

	config.DefaultAuthorName = mergedCfg.AuthorName
	config.DefaultAuthorEmail = mergedCfg.AuthorEmail

	return ctx, cancel, mergedCfg, aiClient, nil
}

func isValidProvider(provider string) bool {
	validProviders := map[string]bool{
		"openai":    true,
		"gemini":    true,
		"anthropic": true,
		"deepseek":  true,
		"phind":     true,
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

	case "phind":
		key, err := config.ResolveAPIKey(phindAPIKeyFlag, "PHIND_API_KEY", cfg.PhindAPIKey, "phind")
		if err != nil && strings.TrimSpace(key) == "" {
			key = ""
		}
		model := cfg.PhindModel
		if modelFlag != "" {
			model = modelFlag
		}
		return phind.NewPhindClient(key, model), nil
	}
	return nil, fmt.Errorf("invalid provider specified: %s", cfg.Provider)
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

	diff, err := git.GetGitDiffIgnoringMoves(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get Git diff (ignoring moves)")
		return
	}
	diff = git.FilterLockFiles(diff, cfg.LockFiles)
	if strings.TrimSpace(diff) == "" {
		fmt.Println("No staged changes after filtering lock files.")
		return
	}

	promptText := prompt.BuildCommitPrompt(diff, languageFlag, commitTypeFlag, "", cfg.PromptTemplate)
	commitMsg, genErr := generateCommitMessage(ctx, aiClient, promptText, commitTypeFlag, templateFlag, cfg.EnableEmoji)
	if genErr != nil {
		log.Error().Err(genErr).Msg("Commit message generation error")
		os.Exit(1)
	}

	var styleReviewSuggestions string
	if reviewMessageFlag {
		suggestions, errReview := enforceCommitMessageStyle(ctx, aiClient, commitMsg, languageFlag, cfg.PromptTemplate)
		if errReview != nil {
			log.Error().Err(errReview).Msg("Commit message style enforcement failed")
			os.Exit(1)
		}
		styleReviewSuggestions = suggestions
	}

	if forceFlag {
		if reviewMessageFlag && strings.TrimSpace(styleReviewSuggestions) != "" &&
			!strings.Contains(strings.ToLower(styleReviewSuggestions), "no issues found") {
			fmt.Println("\nAI Commit Message Style Review Suggestions:")
			fmt.Println(styleReviewSuggestions)
		}
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
		return
	}

	runInteractiveUI(ctx, commitMsg, diff, promptText, styleReviewSuggestions, cfg.EnableEmoji, aiClient)
}

func runAICodeReview(cmd *cobra.Command, args []string) {
	ctx, cancel, cfg, aiClient, err := setupAIEnvironment()
	if err != nil {
		log.Fatal().Err(err).Msg("Setup AI environment error")
		return
	}
	defer cancel()

	diff, err := git.GetGitDiffIgnoringMoves(ctx)
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

func newSummarizeCmd(setupAIEnvironment func() (context.Context, context.CancelFunc, *config.Config, ai.AIClient, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summarize",
		Short: "List commits via fzf, pick one, and summarize the commit with AI",
		Long:  "Displays all commits in a fuzzy finder interface, picks one, and calls the AI provider to produce a summary.",
		Run: func(cmd *cobra.Command, args []string) {
			runSummarizeCommand(setupAIEnvironment)
		},
	}
	return cmd
}

func runSummarizeCommand(setupAIEnvironment func() (context.Context, context.CancelFunc, *config.Config, ai.AIClient, error)) {
	ctx, cancel, cfg, aiClient, err := setupAIEnvironment()
	if err != nil {
		log.Fatal().Err(err).Msg("Setup environment error for summarize command")
		return
	}
	defer cancel()

	if err := summarizer.SummarizeCommits(ctx, aiClient, cfg, languageFlag); err != nil {
		log.Fatal().Err(err).Msg("Failed to summarize commits")
	}
}

func runInteractiveUI(
	ctx context.Context,
	commitMsg string,
	diff string,
	promptText string,
	styleReviewSuggestions string,
	enableEmoji bool,
	aiClient ai.AIClient,
) {
	uiModel := ui.NewUIModel(
		commitMsg,
		diff,
		languageFlag,
		promptText,
		commitTypeFlag,
		templateFlag,
		styleReviewSuggestions,
		enableEmoji,
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

func enforceCommitMessageStyle(
	ctx context.Context,
	client ai.AIClient,
	commitMsg string,
	language string,
	promptTemplate string,
) (string, error) {
	reviewPrompt := prompt.BuildCommitStyleReviewPrompt(commitMsg, language, promptTemplate)
	styleReviewResult, err := client.GetCommitMessage(ctx, reviewPrompt)
	if err != nil {
		return "", fmt.Errorf("commit message style review failed: %w", err)
	}
	return strings.TrimSpace(styleReviewResult), nil
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
