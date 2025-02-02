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

	"errors"

	"github.com/renatogalera/ai-commit/pkg/committypes"
	"github.com/renatogalera/ai-commit/pkg/git"
	"github.com/renatogalera/ai-commit/pkg/openai"
	"github.com/renatogalera/ai-commit/pkg/template"
	"github.com/renatogalera/ai-commit/pkg/ui"
	"github.com/renatogalera/ai-commit/pkg/ui/splitter"
	"github.com/renatogalera/ai-commit/pkg/versioner"
)

type Config struct {
	Prompt           string
	APIKey           string
	CommitType       string
	Template         string
	SemanticRelease  bool
	InteractiveSplit bool
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	apiKeyFlag := flag.String("apiKey", "", "OpenAI API key (or set OPENAI_API_KEY environment variable)")
	languageFlag := flag.String("language", "english", "Language for the commit message")
	commitTypeFlag := flag.String("commit-type", "", "Commit type (e.g. feat, fix, docs)")
	templateFlag := flag.String("template", "", "Commit message template (e.g. \"Modified {GIT_BRANCH} | {COMMIT_MESSAGE}\")")
	forceFlag := flag.Bool("force", false, "Automatically create the commit without prompting")
	semanticReleaseFlag := flag.Bool("semantic-release", false, "Automatically suggest and/or tag a new version based on commit content and run GoReleaser")
	interactiveSplitFlag := flag.Bool("interactive-split", false, "Split your staged changes into multiple commits interactively")

	flag.Parse()

	apiKey := *apiKeyFlag
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		log.Error().Msg("OpenAI API key must be provided via --apiKey flag or OPENAI_API_KEY environment variable")
		os.Exit(1)
	}

	if !git.CheckGitRepository() {
		log.Error().Msg("This is not a git repository")
		os.Exit(1)
	}

	if *commitTypeFlag != "" && !committypes.IsValidCommitType(*commitTypeFlag) {
		log.Error().Msgf("Invalid commit type: %s", *commitTypeFlag)
		os.Exit(1)
	}

	// If user requested interactive split, launch the chunk-splitting TUI
	if *interactiveSplitFlag {
		if err := runInteractiveSplit(apiKey); err != nil {
			log.Error().Err(err).Msg("Error running interactive split")
			os.Exit(1)
		}
		// After successful interactive splitting, we can optionally do semantic release
		if *semanticReleaseFlag {
			headMsg, _ := git.GetHeadCommitMessage()
			cfg := Config{
				APIKey:          apiKey,
				SemanticRelease: true,
			}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			if err := doSemanticRelease(ctx, cfg, headMsg); err != nil {
				log.Error().Err(err).Msg("Error running semantic release")
				os.Exit(1)
			}
		}
		os.Exit(0)
	}

	// Regular single-commit flow:
	diff, err := git.GetGitDiff()
	if err != nil {
		log.Error().Err(err).Msg("Error getting git diff")
		os.Exit(1)
	}

	originalDiff := diff
	diff = git.FilterLockFiles(diff, []string{"go.mod", "go.sum"})
	if strings.TrimSpace(diff) == "" {
		fmt.Println("No changes to commit (after filtering lock files). Did you stage your changes?")
		os.Exit(0)
	}
	if diff != originalDiff {
		fmt.Println("Lock file changes will be committed but not analyzed for commit message generation.")
	}

	truncated := false
	diff, truncated = openai.MaybeSummarizeDiff(diff, 5000)
	if truncated {
		fmt.Println("Note: The diff was truncated for brevity.")
	}

	prompt := openai.BuildPrompt(diff, *languageFlag, *commitTypeFlag)

	cfg := Config{
		Prompt:          prompt,
		APIKey:          apiKey,
		CommitType:      *commitTypeFlag,
		Template:        *templateFlag,
		SemanticRelease: *semanticReleaseFlag,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	commitMsg, err := generateCommitMessage(ctx, cfg)
	if err != nil {
		log.Error().Err(err).Msg("Error generating commit message")
		os.Exit(1)
	}

	if *forceFlag {
		if strings.TrimSpace(commitMsg) == "" {
			log.Error().Msg("Generated commit message is empty")
			os.Exit(1)
		}
		if err := git.CommitChanges(commitMsg); err != nil {
			log.Error().Err(err).Msg("Error creating commit")
			os.Exit(1)
		}
		fmt.Println("Commit created successfully!")

		if cfg.SemanticRelease {
			if err := doSemanticRelease(ctx, cfg, commitMsg); err != nil {
				log.Error().Err(err).Msg("Error running semantic release")
				os.Exit(1)
			}
		}
		os.Exit(0)
	}

	model := ui.NewUIModel(commitMsg, cfg.Prompt, cfg.APIKey, cfg.CommitType, cfg.Template)
	p := ui.NewProgram(model)
	if err := p.Start(); err != nil {
		// If the error is due to a normal cancellation, do not treat it as a failure.
		if errors.Is(err, context.Canceled) {
			os.Exit(0)
		}
		log.Error().Err(err).Msg("Error running TUI program")
		os.Exit(1)
	}

	if cfg.SemanticRelease {
		if err := doSemanticRelease(ctx, cfg, commitMsg); err != nil {
			log.Error().Err(err).Msg("Error running semantic release")
			os.Exit(1)
		}
	}
}

func generateCommitMessage(ctx context.Context, cfg Config) (string, error) {
	msg, err := openai.GetChatCompletion(ctx, cfg.Prompt, cfg.APIKey)
	if err != nil {
		return "", err
	}
	msg = openai.SanitizeOpenAIResponse(msg, cfg.CommitType)
	msg = openai.AddGitmoji(msg, cfg.CommitType)
	if cfg.Template != "" {
		msg, err = template.ApplyTemplate(cfg.Template, msg)
		if err != nil {
			return "", err
		}
	}
	return msg, nil
}

func doSemanticRelease(ctx context.Context, cfg Config, commitMsg string) error {
	log.Info().Msg("Starting semantic release process...")
	currentVersion, err := versioner.GetCurrentVersionTag()
	if err != nil {
		return fmt.Errorf("failed to get current version tag: %w", err)
	}
	if currentVersion == "" {
		log.Info().Msg("No existing version tag found, will assume v0.0.0")
		currentVersion = "v0.0.0"
	}

	suggestedVersion, err := versioner.SuggestNextVersion(ctx, currentVersion, commitMsg, cfg.APIKey)
	if err != nil {
		return fmt.Errorf("failed to suggest next version: %w", err)
	}

	log.Info().Msgf("Suggested next version: %s", suggestedVersion)

	if err := versioner.TagAndPush(suggestedVersion); err != nil {
		return fmt.Errorf("failed to tag and push: %w", err)
	}

	if err := versioner.RunGoReleaser(); err != nil {
		return fmt.Errorf("failed to run goreleaser: %w", err)
	}

	log.Info().Msgf("Semantic release completed: created and pushed tag %s", suggestedVersion)
	return nil
}

func runInteractiveSplit(apiKey string) error {
	diff, err := git.GetGitDiff()
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
		return fmt.Errorf("failed to parse diff: %w", err)
	}

	if len(chunks) == 0 {
		fmt.Println("No diff chunks found.")
		return nil
	}

	model := splitter.NewSplitterModel(chunks, apiKey)
	p := splitter.NewProgram(model)
	if err := p.Start(); err != nil {
		return fmt.Errorf("splitter UI error: %w", err)
	}

	return nil
}
