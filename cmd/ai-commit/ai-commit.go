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

	gogpt "github.com/sashabaranov/go-openai"

	"github.com/renatogalera/ai-commit/pkg/committypes"
	"github.com/renatogalera/ai-commit/pkg/git"
	"github.com/renatogalera/ai-commit/pkg/openai"
	"github.com/renatogalera/ai-commit/pkg/template"
	"github.com/renatogalera/ai-commit/pkg/ui"
	"github.com/renatogalera/ai-commit/pkg/ui/splitter"
	"github.com/renatogalera/ai-commit/pkg/versioner"
)

// defaultTimeout is the timeout used for OpenAI requests. Git commands
// will reuse a smaller or equal context as needed.
const defaultTimeout = 60 * time.Second

// Config holds the configuration values for the commit process.
// We only use this struct locally now, to gather flags. We pass
// only the necessary parameters to each function rather than the
// entire struct.
type Config struct {
	Prompt           string
	CommitType       string
	Template         string
	SemanticRelease  bool
	InteractiveSplit bool
	EnableEmoji      bool
}

// main initializes the application, parses flags, and starts the commit process.
func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	apiKeyFlag := flag.String("apiKey", "", "OpenAI API key (or set OPENAI_API_KEY environment variable)")
	languageFlag := flag.String("language", "english", "Language for the commit message")
	commitTypeFlag := flag.String("commit-type", "", "Commit type (e.g. feat, fix, docs)")
	templateFlag := flag.String("template", "", "Commit message template (e.g. \"Modified {GIT_BRANCH} | {COMMIT_MESSAGE}\")")
	forceFlag := flag.Bool("force", false, "Automatically create the commit without prompting")
	semanticReleaseFlag := flag.Bool("semantic-release", false, "Automatically suggest/tag a new version and run GoReleaser")
	interactiveSplitFlag := flag.Bool("interactive-split", false, "Split your staged changes into multiple commits interactively")
	emojiFlag := flag.Bool("emoji", false, "Include an emoji prefix in the commit message")

	flag.Parse()

	apiKey := *apiKeyFlag
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		log.Error().Msg("OpenAI API key must be provided via --apiKey flag or OPENAI_API_KEY environment variable")
		os.Exit(1)
	}

	// Create a shared OpenAI client for reuse.
	openAIClient := gogpt.NewClient(apiKey)

	// Create a context with timeout for the entire main flow.
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Check if this is a valid Git repo.
	if !git.CheckGitRepository(ctx) {
		log.Error().Msg("This is not a git repository")
		os.Exit(1)
	}

	// Validate commit type if provided.
	if *commitTypeFlag != "" && !committypes.IsValidCommitType(*commitTypeFlag) {
		log.Error().Msgf("Invalid commit type: %s", *commitTypeFlag)
		os.Exit(1)
	}

	// If interactive splitting is requested, run the interactive splitter and exit.
	if *interactiveSplitFlag {
		if err := runInteractiveSplit(ctx, openAIClient); err != nil {
			log.Error().Err(err).Msg("Error running interactive split")
			os.Exit(1)
		}
		// Optionally run semantic release after successful interactive splitting.
		if *semanticReleaseFlag {
			headMsg, _ := git.GetHeadCommitMessage(ctx)
			if err := doSemanticRelease(ctx, openAIClient, headMsg); err != nil {
				log.Error().Err(err).Msg("Error running semantic release")
				os.Exit(1)
			}
		}
		os.Exit(0)
	}

	// Get the staged diff to analyze.
	diff, err := git.GetGitDiff(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Error getting git diff")
		os.Exit(1)
	}

	// Filter out lock files from analysis (but keep them staged).
	originalDiff := diff
	diff = git.FilterLockFiles(diff, []string{"go.mod", "go.sum"})
	if strings.TrimSpace(diff) == "" {
		fmt.Println("No changes to commit (after filtering lock files). Did you stage your changes?")
		os.Exit(0)
	}
	if diff != originalDiff {
		fmt.Println("Lock file changes will be committed but not analyzed for commit message generation.")
	}

	// Possibly truncate the diff if it's huge.
	truncated := false
	diff, truncated = openai.MaybeSummarizeDiff(diff, 5000)
	if truncated {
		fmt.Println("Note: The diff was truncated for brevity.")
	}

	// Build initial prompt (new signature includes extra prompt = "")
	prompt := openai.BuildPrompt(diff, *languageFlag, *commitTypeFlag, "")

	// Prepare local config.
	cfg := Config{
		Prompt:          prompt,
		CommitType:      *commitTypeFlag,
		Template:        *templateFlag,
		SemanticRelease: *semanticReleaseFlag,
		EnableEmoji:     *emojiFlag,
	}

	// Generate commit message using the single openAIClient.
	commitMsg, err := generateCommitMessage(
		ctx,
		openAIClient,
		cfg.Prompt,
		cfg.CommitType,
		cfg.Template,
		cfg.EnableEmoji,
	)
	if err != nil {
		log.Error().Err(err).Msg("Error generating commit message")
		os.Exit(1)
	}

	// If --force is used, commit immediately.
	if *forceFlag {
		if strings.TrimSpace(commitMsg) == "" {
			log.Error().Msg("Generated commit message is empty")
			os.Exit(1)
		}
		if err := git.CommitChanges(ctx, commitMsg); err != nil {
			log.Error().Err(err).Msg("Error creating commit")
			os.Exit(1)
		}
		fmt.Println("Commit created successfully!")

		// Possibly run semantic release after a forced commit.
		if cfg.SemanticRelease {
			if err := doSemanticRelease(ctx, openAIClient, commitMsg); err != nil {
				log.Error().Err(err).Msg("Error running semantic release")
				os.Exit(1)
			}
		}
		os.Exit(0)
	}

	// If we are here, we launch the interactive TUI for commit confirmation/regeneration.
	model := ui.NewUIModel(
		commitMsg,
		diff,          // store the diff
		*languageFlag, // store the language
		cfg.Prompt,
		cfg.CommitType,
		cfg.Template,
		cfg.EnableEmoji,
		openAIClient,
	)
	p := ui.NewProgram(model)
	if err := p.Start(); err != nil {
		if errors.Is(err, context.Canceled) {
			os.Exit(0)
		}
		log.Error().Err(err).Msg("Error running TUI program")
		os.Exit(1)
	}

	// If TUI is done and semantic release is requested, do it now.
	if cfg.SemanticRelease {
		if err := doSemanticRelease(ctx, openAIClient, commitMsg); err != nil {
			log.Error().Err(err).Msg("Error running semantic release")
			os.Exit(1)
		}
	}
}

// generateCommitMessage calls the OpenAI API to generate a commit message.
// This function is decoupled from the config struct, receiving only what it needs.
func generateCommitMessage(
	ctx context.Context,
	client *gogpt.Client,
	prompt string,
	commitType string,
	templateStr string,
	enableEmoji bool,
) (string, error) {
	// Generate from OpenAI
	msg, err := openai.GetChatCompletion(ctx, client, prompt)
	if err != nil {
		return "", err
	}
	// Clean up the message
	msg = openai.SanitizeOpenAIResponse(msg, commitType)
	// Possibly add an emoji prefix
	if enableEmoji {
		msg = openai.AddGitmoji(msg, commitType)
	}
	// Apply the user-defined template
	if templateStr != "" {
		msg, err = template.ApplyTemplate(templateStr, msg)
		if err != nil {
			return "", err
		}
	}
	return msg, nil
}

// doSemanticRelease handles the semantic versioning release process.
// It does not depend on a full config struct, only the parameters needed.
func doSemanticRelease(ctx context.Context, client *gogpt.Client, commitMsg string) error {
	log.Info().Msg("Starting semantic release process...")

	currentVersion, err := versioner.GetCurrentVersionTag(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current version tag: %w", err)
	}
	if currentVersion == "" {
		log.Info().Msg("No existing version tag found, assuming v0.0.0")
		currentVersion = "v0.0.0"
	}

	suggestedVersion, err := versioner.SuggestNextVersion(ctx, currentVersion, commitMsg, client)
	if err != nil {
		return fmt.Errorf("failed to suggest next version: %w", err)
	}

	log.Info().Msgf("Suggested next version: %s", suggestedVersion)

	if err := versioner.TagAndPush(ctx, suggestedVersion); err != nil {
		return fmt.Errorf("failed to tag and push: %w", err)
	}

	if err := versioner.RunGoReleaser(ctx); err != nil {
		return fmt.Errorf("failed to run goreleaser: %w", err)
	}

	log.Info().Msgf("Semantic release completed: created and pushed tag %s", suggestedVersion)
	return nil
}

// runInteractiveSplit launches the interactive UI for splitting diffs into multiple commits.
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
		return fmt.Errorf("failed to parse diff: %w", err)
	}

	if len(chunks) == 0 {
		fmt.Println("No diff chunks found.")
		return nil
	}

	model := splitter.NewSplitterModel(chunks, client)
	p := splitter.NewProgram(model)
	if err := p.Start(); err != nil {
		return fmt.Errorf("splitter UI error: %w", err)
	}

	return nil
}
