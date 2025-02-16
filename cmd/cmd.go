// Package cmd provides additional CLI commands for ai-commit.
package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	gogit "github.com/go-git/go-git/v5"
	gogitobj "github.com/go-git/go-git/v5/plumbing/object"
	"github.com/ktr0731/go-fuzzyfinder"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/renatogalera/ai-commit/pkg/ai"
	"github.com/renatogalera/ai-commit/pkg/config"
)

// NewSummarizeCmd creates the "summarize" command.
// The setupAIEnvironment function is passed from main so that we reuse the existing environment setup.
func NewSummarizeCmd(setupAIEnvironment func() (context.Context, context.CancelFunc, *config.Config, ai.AIClient, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summarize",
		Short: "List commits via fzf, pick one, and summarize the commit with AI",
		Long: `Displays all commits in a fuzzy finder interface; after selecting a commit,
AI-Commit fetches that commit's diff and calls the AI provider to produce a summary.
The resulting output is rendered with a beautiful TUI-like style.`,
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
			// Use humanize.Time to get a relative time string.
			relativeTime := humanize.Time(commit.Author.When)
			// Return commit ID first, then commit message, then the humanized date.
			return fmt.Sprintf("%s | %s | %s", shortHash, firstLine(commit.Message), relativeTime)
		},
		fuzzyfinder.WithPromptString("Select a commit> "),
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

// printFormattedSummary renders the commit summary with a header
// similar to git log --color=always --format='%C(auto)%h%d %s %C(black)%C(bold)%cr'
// and uses lipgloss for styling.
func printFormattedSummary(commit *gogitobj.Commit, summary string) {
	// Define styles
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("63")).
		Underline(true).
		MarginBottom(1)

	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		PaddingLeft(2)

	sectionTitleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212")).
		Underline(true).
		MarginTop(1)

	sectionContentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("250")).
		PaddingLeft(2)

	separatorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	// Header
	fmt.Println(headerStyle.Render("Commit Summary"))
	info := fmt.Sprintf("Short Hash: %s\nAuthor: %s\nDate: %s",
		commit.Hash.String()[:7],
		commit.Author.Name,
		commit.Author.When.Format("Mon Jan 2 15:04:05 MST 2006"))
	fmt.Println(infoStyle.Render(info))
	fmt.Println()

	// Process summary sections. We expect sections separated by "###"
	sections := strings.Split(summary, "###")
	for _, sec := range sections {
		sec = strings.TrimSpace(sec)
		if sec == "" {
			continue
		}
		// The first line is the section title, remaining lines are content.
		lines := strings.SplitN(sec, "\n", 2)
		title := sectionTitleStyle.Render(strings.TrimSpace(lines[0]))
		fmt.Println(title)
		if len(lines) > 1 {
			content := sectionContentStyle.Render(strings.TrimSpace(lines[1]))
			fmt.Println(content)
		}
		fmt.Println()
	}

	fmt.Println(separatorStyle.Render(strings.Repeat("─", 50)))
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
Use "###" to denote section titles. Include:

### General Summary
- Main purpose or key changes

### Detailed Changes
- Any noteworthy details (e.g., new features, bug fixes, refactors)

### Impact and Considerations
- Overview of how it affects the codebase and any considerations.

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
