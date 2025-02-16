package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/ktr0731/go-fuzzyfinder"
	gogit "github.com/go-git/go-git/v5"
	gogitobj "github.com/go-git/go-git/v5/plumbing/object"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/renatogalera/ai-commit/pkg/ai"
	"github.com/renatogalera/ai-commit/pkg/config"
	"github.com/renatogalera/ai-commit/pkg/git"
	"github.com/renatogalera/ai-commit/pkg/prompt"
)

// NewSummarizeCmd creates the "summarize" command.
// The setupAIEnvironment function is passed from main so that we reuse the existing environment setup.
func NewSummarizeCmd(setupAIEnvironment func() (context.Context, context.CancelFunc, *config.Config, ai.AIClient, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summarize",
		Short: "List commits via fzf, pick one, and summarize the commit with AI",
		Long: `Displays all commits in a fuzzy finder interface; after selecting a commit,
AI-Commit fetches that commit's diff and calls the AI provider to produce a summary.`,
		Run: func(cmd *cobra.Command, args []string) {
			runSummarizeCommand(cmd, args, setupAIEnvironment)
		},
	}
	return cmd
}

// runSummarizeCommand sets up the AI environment and calls SummarizeCommits.
func runSummarizeCommand(cmd *cobra.Command, args []string, setupAIEnvironment func() (context.Context, context.CancelFunc, *config.Config, ai.AIClient, error)) {
	ctx, cancel, cfg, aiClient, err := setupAIEnvironment()
	if err != nil {
		log.Fatal().Err(err).Msg("Setup environment error for summarize command")
		return
	}
	defer cancel()

	if err := SummarizeCommits(ctx, aiClient, cfg); err != nil {
		log.Fatal().Err(err).Msg("Failed to summarize commits")
	}
}

// SummarizeCommits lists all commits, allows the user to select one via fzf,
// retrieves its diff, builds a prompt, gets the summary from the AI provider, and prints it.
func SummarizeCommits(ctx context.Context, aiClient ai.AIClient, cfg *config.Config) error {
	repo, err := gogit.PlainOpen(".")
	if err != nil {
		return fmt.Errorf("failed to open git repository: %w", err)
	}

	commits, err := listAllCommits(repo)
	if err != nil {
		return fmt.Errorf("failed to list commits: %w", err)
	}
	if len(commits) == 0 {
		return fmt.Errorf("no commits found in this repository")
	}

	idx, err := fuzzyfinder.Find(
		commits,
		func(i int) string {
			commit := commits[i]
			shortHash := commit.Hash.String()[:7]
			// Changed the order so that the commit message comes first
			return fmt.Sprintf("%s | %s", firstLine(commit.Message), shortHash)
		},
		fuzzyfinder.WithPromptString("Select a commit to summarize> "),
	)
	if err != nil {
		return fmt.Errorf("fuzzyfinder error: %w", err)
	}

	selectedCommit := commits[idx]
	diffStr, err := getCommitDiff(repo, selectedCommit)
	if err != nil {
		return fmt.Errorf("failed to get commit diff: %w", err)
	}

	if strings.TrimSpace(diffStr) == "" {
		fmt.Println("No diff found for this commit (maybe an empty commit or merge commit?).")
		return nil
	}

	commitSummaryPrompt := buildCommitSummaryPrompt(selectedCommit, diffStr, cfg.PromptTemplate)
	summary, err := aiClient.GetCommitMessage(ctx, commitSummaryPrompt)
	if err != nil {
		return fmt.Errorf("failed to summarize commit with AI: %w", err)
	}

	summary = aiClient.SanitizeResponse(summary, "")
	printFormattedSummary(selectedCommit, summary)

	return nil
}

// printFormattedSummary displays the commit summary with formatted sections.
func printFormattedSummary(commit *gogitobj.Commit, summary string) {
	fmt.Println("\n## Commit Summary")

	shortHash := commit.Hash.String()[:7]
	author := commit.Author.Name
	// Use a standard date format.
	date := commit.Author.When.Format("Mon Jan 2 15:04:05 MST 2006")

	fmt.Printf("* **Short Hash:** `%s`\n", shortHash)
	fmt.Printf("* **Author:** %s\n", author)
	fmt.Printf("* **Date:** %s\n\n", date)

	sections := strings.Split(summary, "##")
	for _, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}
		lines := strings.SplitN(section, "\n", 2)
		title := strings.TrimSpace(lines[0])
		content := ""
		if len(lines) > 1 {
			content = strings.TrimSpace(lines[1])
		}

		if title != "" {
			fmt.Printf("### %s\n", title)
		}
		if content != "" {
			fmt.Println(content + "\n")
		}
	}

	fmt.Println("---")
}

// listAllCommits retrieves all commits from the repository.
func listAllCommits(repo *gogit.Repository) ([]*gogitobj.Commit, error) {
	headRef, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("cannot find HEAD: %w", err)
	}

	commitIter, err := repo.Log(&gogit.LogOptions{From: headRef.Hash()})
	if err != nil {
		return nil, fmt.Errorf("failed to get commit log: %w", err)
	}
	defer commitIter.Close()

	var commits []*gogitobj.Commit
	err = commitIter.ForEach(func(c *gogitobj.Commit) error {
		commits = append(commits, c)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("unable to iterate commits: %w", err)
	}

	return commits, nil
}

// getCommitDiff obtains the diff for the specified commit.
func getCommitDiff(repo *gogit.Repository, commit *gogitobj.Commit) (string, error) {
	if commit.NumParents() == 0 {
		tree, err := commit.Tree()
		if err != nil {
			return "", err
		}
		return getDiffAgainstEmpty(tree)
	}

	parent, err := commit.Parent(0)
	if err != nil {
		return "", err
	}
	patch, err := parent.Patch(commit)
	if err != nil {
		return "", err
	}
	return patch.String(), nil
}

// getDiffAgainstEmpty handles the diff for the initial commit.
func getDiffAgainstEmpty(commitTree *gogitobj.Tree) (string, error) {
	emptyTree := &gogitobj.Tree{}
	patch, err := emptyTree.Patch(commitTree)
	if err != nil {
		return "", err
	}
	return patch.String(), nil
}

// buildCommitSummaryPrompt constructs the prompt used to ask the AI for a summary.
func buildCommitSummaryPrompt(commit *gogitobj.Commit, diffStr, customPromptTemplate string) string {
	defaultTemplate := `Summarize the following git commit in markdown format.
Use "## " for section titles. Include:

## General Summary
- Main purpose or key changes

## Detailed Changes
- Any noteworthy details (e.g., new features, bug fixes, refactors)

## Impact and Considerations
- High-level overview of how it affects the codebase and other important considerations.

Commit Information:
Author: {AUTHOR}
Date: {DATE}
Commit Message:
{COMMIT_MSG}

Diff:
{DIFF}
`
	templateUsed := defaultTemplate
	if strings.TrimSpace(customPromptTemplate) != "" {
		templateUsed = customPromptTemplate
	}

	promptText := strings.ReplaceAll(templateUsed, "{AUTHOR}", commit.Author.Name)
	promptText = strings.ReplaceAll(promptText, "{DATE}", commit.Author.When.Format("Mon Jan 2 15:04:05 MST 2006"))
	promptText = strings.ReplaceAll(promptText, "{COMMIT_MSG}", commit.Message)
	promptText = strings.ReplaceAll(promptText, "{DIFF}", diffStr)

	return promptText
}

// firstLine returns the first line of a given string.
func firstLine(msg string) string {
	lines := strings.Split(msg, "\n")
	return strings.TrimSpace(lines[0])
}

