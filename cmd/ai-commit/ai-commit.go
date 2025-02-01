package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/renatogalera/ai-commit/pkg/git"
	"github.com/renatogalera/ai-commit/pkg/openai"
	"github.com/renatogalera/ai-commit/pkg/template"
	"github.com/renatogalera/ai-commit/pkg/ui"
)

type Config struct {
	Prompt     string
	APIKey     string
	CommitType string
	Template   string
}

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	apiKeyFlag := flag.String("apiKey", "", "OpenAI API key (or set OPENAI_API_KEY environment variable)")
	languageFlag := flag.String("language", "english", "Language for the commit message")
	commitTypeFlag := flag.String("commit-type", "", "Commit type (e.g. feat, fix, docs)")
	templateFlag := flag.String("template", "", "Commit message template (e.g. \"Modified {GIT_BRANCH} | {COMMIT_MESSAGE}\")")
	forceFlag := flag.Bool("force", false, "Automatically create the commit without prompting")
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

	diff, err := git.GetGitDiff()
	if err != nil {
		log.Error().Err(err).Msg("Error getting git diff")
		os.Exit(1)
	}

	originalDiff := diff
	diff = git.FilterLockFiles(diff)
	if strings.TrimSpace(diff) == "" {
		fmt.Println("No changes to commit (after filtering lock files). Did you stage your changes?")
		os.Exit(0)
	}
	if diff != originalDiff {
		fmt.Println("Lock file changes will be committed but not analyzed for commit message generation.")
	}

	diff = openai.MaybeSummarizeDiff(diff, 5000)
	prompt := openai.BuildPrompt(diff, *languageFlag, *commitTypeFlag)

	cfg := Config{
		Prompt:     prompt,
		APIKey:     apiKey,
		CommitType: *commitTypeFlag,
		Template:   *templateFlag,
	}

	commitMsg, err := generateCommitMessage(cfg)
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
		os.Exit(0)
	}

	model := ui.NewUIModel(commitMsg, cfg.Prompt, cfg.APIKey, cfg.CommitType, cfg.Template)
	p := ui.NewProgram(model)
	if err := p.Start(); err != nil {
		log.Error().Err(err).Msg("Error running TUI program")
		os.Exit(1)
	}
}

func generateCommitMessage(cfg Config) (string, error) {
	msg, err := openai.GetChatCompletion(cfg.Prompt, cfg.APIKey)
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
