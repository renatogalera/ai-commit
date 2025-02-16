package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

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

func main() {
	// Initialize logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	// Load or create configuration
	cfg, err := config.LoadOrCreateConfig()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load configuration")
		os.Exit(1)
	}

	// Set Git commit author details
	git.CommitAuthorName = cfg.AuthorName
	git.CommitAuthorEmail = cfg.AuthorEmail

	// Parse CLI flags
	apiKeyFlag := flag.String("apiKey", "", "API key for OpenAI provider")
	geminiAPIKeyFlag := flag.String("geminiApiKey", cfg.GeminiAPIKey, "API key for Gemini provider")
	anthropicAPIKeyFlag := flag.String("anthropicApiKey", cfg.AnthropicAPIKey, "API key for Anthropic provider")
	deepseekAPIKeyFlag := flag.String("deepseekApiKey", cfg.DeepseekAPIKey, "API key for Deepseek provider")
	languageFlag := flag.String("language", "english", "Language for commit message")
	commitTypeFlag := flag.String("commit-type", cfg.CommitType, "Commit type (e.g., feat, fix)")
	templateFlag := flag.String("template", cfg.Template, "Commit message template")
	forceFlag := flag.Bool("force", false, "Bypass interactive UI and commit directly")
	semanticReleaseFlag := flag.Bool("semantic-release", cfg.SemanticRelease, "Perform semantic release")
	interactiveSplitFlag := flag.Bool("interactive-split", cfg.InteractiveSplit, "Launch interactive commit splitting")
	emojiFlag := flag.Bool("emoji", cfg.EnableEmoji, "Include emoji in commit message")
	manualSemverFlag := flag.Bool("manual-semver", false, "Manually select semantic version bump")
	providerFlag := flag.String("provider", cfg.Provider, "AI provider: openai, gemini, anthropic, deepseek")
	modelFlag := flag.String("model", "", "Sub-model for the chosen provider")
	flag.Parse()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Initialize AI client
	aiClient, err := initAIClient(ctx, cfg, *providerFlag, *apiKeyFlag, *modelFlag, *geminiAPIKeyFlag, *anthropicAPIKeyFlag, *deepseekAPIKeyFlag)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize AI client")
		os.Exit(1)
	}

	// Verify Git repository
	if !git.IsGitRepository(ctx) {
		log.Error().Msg("Not a valid Git repository")
		os.Exit(1)
	}

	// Validate commit type if provided
	if *commitTypeFlag != "" && !committypes.IsValidCommitType(*commitTypeFlag) {
		log.Error().Msgf("Invalid commit type: %s", *commitTypeFlag)
		os.Exit(1)
	}

	// Interactive split flow
	if *interactiveSplitFlag {
		if err := splitter.RunInteractiveSplit(ctx, aiClient); err != nil {
			log.Error().Err(err).Msg("Interactive split failed")
			os.Exit(1)
		}
		if *semanticReleaseFlag {
			headMsg, _ := git.GetHeadCommitMessage(ctx)
			if err := versioner.PerformSemanticRelease(ctx, aiClient, headMsg, *manualSemverFlag); err != nil {
				log.Error().Err(err).Msg("Semantic release failed")
				os.Exit(1)
			}
		}
		os.Exit(0)
	}

	// Retrieve and filter Git diff
	diff, err := git.GetGitDiff(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get Git diff")
		os.Exit(1)
	}

	originalDiff := diff

	diff = git.FilterLockFiles(diff, []string{"go.mod", "go.sum"})
	if strings.TrimSpace(diff) == "" {
		fmt.Println("No staged changes found after filtering lock files.")
		os.Exit(0)
	}
	if diff != originalDiff {
		fmt.Println("Note: Lock file changes are not used for AI generation but will be committed.")
	}

	if strings.TrimSpace(diff) == "" {
		fmt.Println("No staged changes found.")
		os.Exit(0)
	}

	// Build AI prompt
	promptText := prompt.BuildPrompt(diff, *languageFlag, *commitTypeFlag, "")

	// Generate commit message
	commitMsg, err := generateCommitMessage(ctx, aiClient, promptText, *commitTypeFlag, *templateFlag, *emojiFlag)
	if err != nil {
		log.Error().Err(err).Msg("Commit message generation failed")
		os.Exit(1)
	}

	// Force commit without UI if requested
	if *forceFlag {
		if strings.TrimSpace(commitMsg) == "" {
			log.Error().Msg("Generated commit message is empty; aborting commit.")
			os.Exit(1)
		}
		if err := git.CommitChanges(ctx, commitMsg); err != nil {
			log.Error().Err(err).Msg("Commit failed")
			os.Exit(1)
		}
		fmt.Println("Commit created successfully (forced).")
		if *semanticReleaseFlag {
			if err := versioner.PerformSemanticRelease(ctx, aiClient, commitMsg, *manualSemverFlag); err != nil {
				log.Error().Err(err).Msg("Semantic release failed")
				os.Exit(1)
			}
		}
		os.Exit(0)
	}

	// Launch interactive UI for commit editing
	uiModel := ui.NewUIModel(commitMsg, diff, *languageFlag, promptText, *commitTypeFlag, *templateFlag, *emojiFlag, aiClient)
	program := ui.NewProgram(uiModel)
	if _, err := program.Run(); err != nil {
		log.Error().Err(err).Msg("UI encountered an error")
		os.Exit(1)
	}

	// Perform semantic release if enabled
	if *semanticReleaseFlag {
		if err := versioner.PerformSemanticRelease(ctx, aiClient, commitMsg, *manualSemverFlag); err != nil {
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
	msg = ai.SanitizeResponse(msg, commitType)
	if enableEmoji {
		msg = git.AddGitmoji(msg, commitType)
	}
	if tmpl != "" {
		msg, err = template.ApplyTemplate(tmpl, msg)
		if err != nil {
			return "", err
		}
	}
	return strings.TrimSpace(msg), nil
}

// initAIClient initializes the appropriate AI client based on configuration and flags.
func initAIClient(
	ctx context.Context,
	cfg *config.Config,
	provider, apiKey, model, geminiKey, anthropicKey, deepseekKey string,
) (ai.AIClient, error) {

	provider = strings.TrimSpace(provider)
	if provider == "" {
		provider = cfg.Provider
	}

	switch provider {
	case "openai":
		key, err := config.ResolveAPIKey(apiKey, "OPENAI_API_KEY", cfg.OpenAIAPIKey, "openai")
		if err != nil {
			return nil, err
		}
		if model == "" {
			model = cfg.OpenAIModel
		}
		return openai.NewOpenAIClient(key, model), nil

	case "gemini":
		key, err := config.ResolveAPIKey(geminiKey, "GEMINI_API_KEY", cfg.GeminiAPIKey, "gemini")
		if err != nil {
			return nil, err
		}
		if model == "" {
			model = cfg.GeminiModel
		}
		geminiClient, err := gemini.NewGeminiProClient(ctx, key, model)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Gemini client: %w", err)
		}
		return gemini.NewClient(geminiClient), nil

	case "anthropic":
		key, err := config.ResolveAPIKey(anthropicKey, "ANTHROPIC_API_KEY", cfg.AnthropicAPIKey, "anthropic")
		if err != nil {
			return nil, err
		}
		if model == "" {
			model = cfg.AnthropicModel
		}
		anthroClient, err := anthropic.NewAnthropicClient(key, model)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Anthropic client: %w", err)
		}
		return anthroClient, nil

	case "deepseek":
		key, err := config.ResolveAPIKey(deepseekKey, "DEEPSEEK_API_KEY", cfg.DeepseekAPIKey, "deepseek")
		if err != nil {
			return nil, err
		}
		if model == "" {
			model = cfg.DeepseekModel
		}
		deepseekClient, err := deepseek.NewDeepseekClient(key, model)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Deepseek client: %w", err)
		}
		return deepseekClient, nil

	default:
		return nil, fmt.Errorf("invalid provider specified: %s", provider)
	}
}
