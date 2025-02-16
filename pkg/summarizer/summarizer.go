package summarizer

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	gogit "github.com/go-git/go-git/v5"
	gogitobj "github.com/go-git/go-git/v5/plumbing/object"
	"github.com/ktr0731/go-fuzzyfinder"
	"github.com/renatogalera/ai-commit/pkg/ai"
	"github.com/renatogalera/ai-commit/pkg/config"
	"github.com/renatogalera/ai-commit/pkg/prompt"
)

// SummarizeCommits lists all commits in the current repository, allows the user to pick one via a fuzzy finder,
// retrieves its diff, builds an AI prompt, and prints the AI-generated summary.
func SummarizeCommits(ctx context.Context, aiClient ai.AIClient, cfg *config.Config) error {
	// Open the current git repository.
	repo, err := gogit.PlainOpen(".")
	if err != nil {
		return fmt.Errorf("failed to open git repository: %w", err)
	}

	// List all commits.
	commits, err := listAllCommits(repo)
	if err != nil {
		return fmt.Errorf("failed to list commits: %w", err)
	}
	if len(commits) == 0 {
		return fmt.Errorf("no commits found in this repository")
	}

	// Use fuzzyfinder to let the user select a commit.
	idx, err := fuzzyfinder.Find(
		commits,
		func(i int) string {
			commit := commits[i]
			shortHash := commit.Hash.String()[:7]
			relativeTime := humanize.Time(commit.Author.When)
			return fmt.Sprintf("%s | %s | %s", shortHash, firstLine(commit.Message), relativeTime)
		},
		fuzzyfinder.WithPromptString("Select a commit> "),
	)
	if err != nil {
		return fmt.Errorf("fuzzyfinder error: %w", err)
	}

	// Get the selected commit and its diff.
	selectedCommit := commits[idx]
	diffStr, err := getCommitDiff(repo, selectedCommit)
	if err != nil {
		return fmt.Errorf("failed to get commit diff: %w", err)
	}
	if strings.TrimSpace(diffStr) == "" {
		fmt.Println("No diff found for this commit (maybe an empty or merge commit).")
		return nil
	}

	// Build the prompt for the AI using the commit diff.
	commitSummaryPrompt := prompt.BuildCommitSummaryPrompt(selectedCommit, diffStr, cfg.PromptTemplate)
	summary, err := aiClient.GetCommitMessage(ctx, commitSummaryPrompt)
	if err != nil {
		return fmt.Errorf("failed to summarize commit with AI: %w", err)
	}
	summary = aiClient.SanitizeResponse(summary, "")

	// Print the formatted summary.
	printFormattedSummary(selectedCommit, summary)
	return nil
}

// printFormattedSummary renders the commit summary with styling.
func printFormattedSummary(commit *gogitobj.Commit, summary string) {
	// Define styles.
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

	// Render header.
	fmt.Println(headerStyle.Render("Commit Summary"))
	info := fmt.Sprintf("Short Hash: %s\nAuthor: %s\nDate: %s",
		commit.Hash.String()[:7],
		commit.Author.Name,
		commit.Author.When.Format("Mon Jan 2 15:04:05 MST 2006"))
	fmt.Println(infoStyle.Render(info))
	fmt.Println()

	// Process summary sections (expecting sections separated by "###").
	sections := strings.Split(summary, "###")
	for _, sec := range sections {
		sec = strings.TrimSpace(sec)
		if sec == "" {
			continue
		}
		// The first line is the section title; the rest is the content.
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

// getCommitDiff obtains the diff for a given commit.
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

// getDiffAgainstEmpty handles diff generation for the initial commit.
func getDiffAgainstEmpty(commitTree *gogitobj.Tree) (string, error) {
	emptyTree := &gogitobj.Tree{}
	patch, err := emptyTree.Patch(commitTree)
	if err != nil {
		return "", err
	}
	return patch.String(), nil
}

// firstLine returns the first non-empty line from a string.
func firstLine(msg string) string {
	lines := strings.Split(msg, "\n")
	return strings.TrimSpace(lines[0])
}
