package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	gogpt "github.com/sashabaranov/go-openai"

	"github.com/renatogalera/ai-commit/pkg/ai"
	"github.com/renatogalera/ai-commit/pkg/committypes"
	"github.com/renatogalera/ai-commit/pkg/gemini"
	"github.com/renatogalera/ai-commit/pkg/git"
	"github.com/renatogalera/ai-commit/pkg/openai"
	"github.com/renatogalera/ai-commit/pkg/prompt"
	"github.com/renatogalera/ai-commit/pkg/template"
	"github.com/renatogalera/ai-commit/pkg/ui"
	"github.com/renatogalera/ai-commit/pkg/ui/splitter"
	"github.com/renatogalera/ai-commit/pkg/versioner"
)

const defaultTimeout = 60 * time.Second

type Config struct {
	Prompt           string
	CommitType       string
	Template         string
	SemanticRelease  bool
	InteractiveSplit bool
	EnableEmoji      bool
	ModelName        string
	GeminiAPIKey     string
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	apiKeyFlag := flag.String("apiKey", "", "OpenAI API key (or set OPENAI_API_KEY environment variable)")
	languageFlag := flag.String("language", "english", "Language for the commit message")
	commitTypeFlag := flag.String("commit-type", "", "Commit type (e.g. feat, fix, docs)")
	templateFlag := flag.String("template", "", "Commit message template (e.g. 'Modified {GIT_BRANCH} | {COMMIT_MESSAGE}')")
	forceFlag := flag.Bool("force", false, "Automatically commit without TUI")
	semanticReleaseFlag := flag.Bool("semantic-release", false, "Suggest/tag a new version")
	interactiveSplitFlag := flag.Bool("interactive-split", false, "Interactively split staged changes into multiple commits")
	emojiFlag := flag.Bool("emoji", false, "Include an emoji prefix in commit message")
	manualSemverFlag := flag.Bool("manual-semver", false, "Pick the next version manually (major/minor/patch) instead of AI suggestion")
	modelFlag := flag.String("model", "openai", "AI model to use (openai or gemini)")
	geminiAPIKeyFlag := flag.String("geminiApiKey", "", "Google Gemini API key (or set GEMINI_API_KEY environment variable)")

	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	var aiClient ai.AIClient
	var modelName string
	var apiKey string

	modelName = *modelFlag
	if modelName != "openai" && modelName != "gemini" {
		log.Error().Msg("Invalid model specified. Choose 'openai' or 'gemini'.")
		os.Exit(1)
	}

	if modelName == "openai" {
		apiKey = *apiKeyFlag
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		if apiKey == "" {
			log.Error().Msg("OpenAI API key is required (flag --apiKey or env OPENAI_API_KEY) when using OpenAI model.")
			os.Exit(1)
		}
		openAIClient := gogpt.NewClient(apiKey)
		aiClient = openai.NewOpenAIClient(openAIClient)
	} else if modelName == "gemini" {
		apiKey = *geminiAPIKeyFlag
		if apiKey == "" {
			apiKey = os.Getenv("GEMINI_API_KEY")
		}
		if apiKey == "" {
			log.Error().Msg("Gemini API key is required (flag --geminiApiKey or env GEMINI_API_KEY) when using Gemini model.")
			os.Exit(1)
		}
		geminiClient, err := gemini.NewGeminiProClient(ctx, apiKey)
		if err != nil {
			log.Error().Err(err).Msg("Failed to initialize Gemini client")
			os.Exit(1)
		}
		aiClient = gemini.NewClient(geminiClient)
	} else {
		log.Error().Msg("No AI model selected.")
		os.Exit(1)
	}

	if !git.CheckGitRepository(ctx) {
		log.Error().Msg("This is not a Git repository.")
		os.Exit(1)
	}

	if *commitTypeFlag != "" && !committypes.IsValidCommitType(*commitTypeFlag) {
		log.Error().Msgf("Invalid commit type: %s", *commitTypeFlag)
		os.Exit(1)
	}

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

	diff, err := git.GetGitDiff(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Error getting Git diff")
		os.Exit(1)
	}

	originalDiff := diff
	diff = git.FilterLockFiles(diff, []string{"go.mod", "go.sum"})
	if strings.TrimSpace(diff) == "" {
		fmt.Println("No changes to commit (after filtering lock files). Did you stage your changes?")
		os.Exit(0)
	}
	if diff != originalDiff {
		fmt.Println("Note: lock file changes are committed but not analyzed for AI commit message generation.")
	}

	diff, _ = openai.MaybeSummarizeDiff(diff, 5000)

	promptText := prompt.BuildPrompt(diff, *languageFlag, *commitTypeFlag, "")

	cfg := Config{
		Prompt:          promptText,
		CommitType:      *commitTypeFlag,
		Template:        *templateFlag,
		SemanticRelease: *semanticReleaseFlag,
		EnableEmoji:     *emojiFlag,
		ModelName:       modelName,
		GeminiAPIKey:    *geminiAPIKeyFlag,
	}

	commitMsg, err := generateCommitMessage(ctx, aiClient, cfg.Prompt, cfg.CommitType, cfg.Template, cfg.EnableEmoji)
	if err != nil {
		log.Error().Err(err).Msg("Error generating commit message")
		os.Exit(1)
	}

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
		if cfg.SemanticRelease {
			if err := doSemanticRelease(ctx, aiClient, commitMsg, *manualSemverFlag); err != nil {
				log.Error().Err(err).Msg("Error in semantic release")
				os.Exit(1)
			}
		}
		os.Exit(0)
	}

	model := ui.NewUIModel(
		commitMsg,
		diff,
		*languageFlag,
		promptText,
		*commitTypeFlag,
		cfg.Template,
		cfg.EnableEmoji,
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

	if cfg.SemanticRelease {
		if err := doSemanticRelease(ctx, aiClient, commitMsg, *manualSemverFlag); err != nil {
			log.Error().Err(err).Msg("Error in semantic release")
			os.Exit(1)
		}
	}
}

func generateCommitMessage(ctx context.Context, client ai.AIClient, prompt string, commitType string, templateStr string, enableEmoji bool) (string, error) {
	res, err := client.GetCommitMessage(ctx, prompt)
	if err != nil {
		return "", err
	}
	res = openai.SanitizeOpenAIResponse(res, commitType)
	if enableEmoji {
		res = openai.AddGitmoji(res, commitType)
	}
	if templateStr != "" {
		res, err = template.ApplyTemplate(templateStr, res)
		if err != nil {
			return "", err
		}
	}
	return res, nil
}

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
	if err := prog.Start(); err != nil {
		return fmt.Errorf("splitter UI error: %w", err)
	}
	return nil
}
