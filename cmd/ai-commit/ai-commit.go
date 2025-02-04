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

	"github.com/renatogalera/ai-commit/pkg/committypes"
	"github.com/renatogalera/ai-commit/pkg/git"
	"github.com/renatogalera/ai-commit/pkg/openai"
	"github.com/renatogalera/ai-commit/pkg/template"
	"github.com/renatogalera/ai-commit/pkg/ui"
	"github.com/renatogalera/ai-commit/pkg/ui/splitter"
	"github.com/renatogalera/ai-commit/pkg/versioner"
)

const defaultTimeout = 60 * time.Second

// Config is just a small struct to hold the relevant flags for clarity.
type Config struct {
	Prompt           string
	CommitType       string
	Template         string
	SemanticRelease  bool
	InteractiveSplit bool
	EnableEmoji      bool
}

// main parses flags and orchestrates the commit workflow.
func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	apiKeyFlag := flag.String("apiKey", "", "OpenAI API key (or set OPENAI_API_KEY environment variable)")
	languageFlag := flag.String("language", "english", "Language for the commit message")
	commitTypeFlag := flag.String("commit-type", "", "Commit type (e.g. feat, fix, docs)")
	templateFlag := flag.String("template", "", "Commit message template (e.g. 'Modified {GIT_BRANCH} | {COMMIT_MESSAGE}')")
	forceFlag := flag.Bool("force", false, "Automatically commit without TUI")
	semanticReleaseFlag := flag.Bool("semantic-release", false, "Suggest/tag a new version + run GoReleaser after commit")
	interactiveSplitFlag := flag.Bool("interactive-split", false, "Interactively split staged changes into multiple commits")
	emojiFlag := flag.Bool("emoji", false, "Include an emoji prefix in commit message")
	manualSemverFlag := flag.Bool("manual-semver", false, "Pick the next version manually (major/minor/patch) instead of AI suggestion")

	flag.Parse()

	// Check for OpenAI API key
	apiKey := *apiKeyFlag
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		log.Error().Msg("OpenAI API key is required (flag --apiKey or env OPENAI_API_KEY).")
		os.Exit(1)
	}

	// Create one shared OpenAI client
	openAIClient := gogpt.NewClient(apiKey)

	// Create a context for everything
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Ensure we're in a Git repo
	if !git.CheckGitRepository(ctx) {
		log.Error().Msg("This is not a Git repository.")
		os.Exit(1)
	}

	// Validate commitType if provided
	if *commitTypeFlag != "" && !committypes.IsValidCommitType(*commitTypeFlag) {
		log.Error().Msgf("Invalid commit type: %s", *commitTypeFlag)
		os.Exit(1)
	}

	// Interactive Split Flow
	if *interactiveSplitFlag {
		if err := runInteractiveSplit(ctx, openAIClient); err != nil {
			log.Error().Err(err).Msg("Error in interactive split")
			os.Exit(1)
		}
		// Optionally do semantic release after splitting
		if *semanticReleaseFlag {
			headMsg, _ := git.GetHeadCommitMessage(ctx)
			if err := doSemanticRelease(ctx, openAIClient, headMsg, *manualSemverFlag); err != nil {
				log.Error().Err(err).Msg("Error in semantic release")
				os.Exit(1)
			}
		}
		os.Exit(0)
	}

	// Standard AI commit flow
	diff, err := git.GetGitDiff(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Error getting Git diff")
		os.Exit(1)
	}

	// Filter out lock files from analysis
	originalDiff := diff
	diff = git.FilterLockFiles(diff, []string{"go.mod", "go.sum"})
	if strings.TrimSpace(diff) == "" {
		fmt.Println("No changes to commit (after filtering lock files). Did you stage your changes?")
		os.Exit(0)
	}
	if diff != originalDiff {
		fmt.Println("Note: lock file changes are committed but not analyzed for AI commit message generation.")
	}

	// Possibly truncate the diff for OpenAI
	truncated := false
	diff, truncated = openai.MaybeSummarizeDiff(diff, 5000)
	if truncated {
		fmt.Println("Note: Diff was truncated for brevity.")
	}

	// Build the prompt for OpenAI
	prompt := openai.BuildPrompt(diff, *languageFlag, *commitTypeFlag, "")

	cfg := Config{
		Prompt:          prompt,
		CommitType:      *commitTypeFlag,
		Template:        *templateFlag,
		SemanticRelease: *semanticReleaseFlag,
		EnableEmoji:     *emojiFlag,
	}

	// Generate commit message
	commitMsg, err := generateCommitMessage(ctx, openAIClient, cfg.Prompt, cfg.CommitType, cfg.Template, cfg.EnableEmoji)
	if err != nil {
		log.Error().Err(err).Msg("Error generating commit message")
		os.Exit(1)
	}

	// If --force, commit immediately
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
			if err := doSemanticRelease(ctx, openAIClient, commitMsg, *manualSemverFlag); err != nil {
				log.Error().Err(err).Msg("Error in semantic release")
				os.Exit(1)
			}
		}
		os.Exit(0)
	}

	// Otherwise, run the TUI to allow user to confirm/regenerate
	model := ui.NewUIModel(
		commitMsg,
		diff,
		*languageFlag,
		cfg.Prompt,
		cfg.CommitType,
		cfg.Template,
		cfg.EnableEmoji,
		openAIClient,
	)
	p := ui.NewProgram(model)
	if err := p.Start(); err != nil {
		// If user canceled or error
		if errors.Is(err, context.Canceled) {
			os.Exit(0)
		}
		log.Error().Err(err).Msg("TUI error")
		os.Exit(1)
	}

	// After TUI is done, do semantic release if requested
	if cfg.SemanticRelease {
		if err := doSemanticRelease(ctx, openAIClient, commitMsg, *manualSemverFlag); err != nil {
			log.Error().Err(err).Msg("Error in semantic release")
			os.Exit(1)
		}
	}
}

// generateCommitMessage calls OpenAI to get a commit message, then sanitizes it.
func generateCommitMessage(
	ctx context.Context,
	client *gogpt.Client,
	prompt string,
	commitType string,
	templateStr string,
	enableEmoji bool,
) (string, error) {
	res, err := openai.GetChatCompletion(ctx, client, prompt)
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

// doSemanticRelease handles the version bump logic (AI or manual).
func doSemanticRelease(ctx context.Context, client *gogpt.Client, commitMsg string, manual bool) error {
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
		// Use the TUI approach from versioner/semver_tui
		userPicked, err := versioner.RunSemVerTUI(ctx, currentVersion)
		if err != nil {
			return fmt.Errorf("manual semver TUI error: %w", err)
		}
		if userPicked != "" {
			nextVersion = userPicked
			log.Info().Msgf("User selected next version: %s", nextVersion)
		} else {
			// If user pressed q, fallback to AI suggestion
			aiVer, aiErr := versioner.SuggestNextVersion(ctx, currentVersion, commitMsg, client)
			if aiErr != nil {
				return fmt.Errorf("failed AI suggestion: %w", aiErr)
			}
			nextVersion = aiVer
			log.Info().Msgf("No manual selection, fallback AI version: %s", nextVersion)
		}
	} else {
		// Normal AI suggestion
		aiVer, aiErr := versioner.SuggestNextVersion(ctx, currentVersion, commitMsg, client)
		if aiErr != nil {
			return fmt.Errorf("AI version suggestion error: %w", aiErr)
		}
		nextVersion = aiVer
		log.Info().Msgf("AI-suggested version: %s", nextVersion)
	}

	if err := versioner.TagAndPush(ctx, nextVersion); err != nil {
		return fmt.Errorf("failed to tag and push %s: %w", nextVersion, err)
	}

	if err := versioner.RunGoReleaser(ctx); err != nil {
		return fmt.Errorf("goreleaser failed: %w", err)
	}

	log.Info().Msgf("Semantic release done! Pushed tag %s", nextVersion)
	return nil
}

// runInteractiveSplit starts the partial-commit TUI from pkg/ui/splitter.
func runInteractiveSplit(ctx context.Context, client *gogpt.Client) error {
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
