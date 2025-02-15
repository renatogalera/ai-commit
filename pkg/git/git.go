package git

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/renatogalera/ai-commit/pkg/committypes"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// Global variables to hold commit author details.
// These are set from the config in main().
var (
	// CommitAuthorName holds the commit author's name for git commits.
	CommitAuthorName = "ai-commit"
	// CommitAuthorEmail holds the commit author's email for git commits.
	CommitAuthorEmail = "ai-commit@example.com"
)

// isBinary determines whether the provided data is binary.
func isBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	return bytes.IndexByte(data, 0) != -1
}

// ParseDiffToChunks splits a git diff string into chunks.
func ParseDiffToChunks(diff string) ([]DiffChunk, error) {
	lines := strings.Split(diff, "\n")
	var chunks []DiffChunk
	var currentChunk *DiffChunk
	var currentFile string
	var inHunk bool
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			if currentChunk != nil {
				chunks = append(chunks, *currentChunk)
				currentChunk = nil
			}
			file := parseFilePath(line)
			if file != "" {
				currentFile = file
			}
			inHunk = false
			continue
		}
		if strings.HasPrefix(line, "@@ ") {
			if currentChunk != nil {
				chunks = append(chunks, *currentChunk)
			}
			currentChunk = &DiffChunk{
				FilePath:   currentFile,
				HunkHeader: line,
				Lines:      []string{},
			}
			inHunk = true
			continue
		}
		if inHunk && currentChunk != nil {
			currentChunk.Lines = append(currentChunk.Lines, line)
		}
	}
	if currentChunk != nil {
		chunks = append(chunks, *currentChunk)
	}
	return chunks, nil
}

// DiffChunk represents a section of a git diff.
type DiffChunk struct {
	FilePath   string
	HunkHeader string
	Lines      []string
}

// parseFilePath extracts the file path from a diff header line.
func parseFilePath(diffLine string) string {
	parts := strings.Split(diffLine, " ")
	if len(parts) < 4 {
		return ""
	}
	aPath := strings.TrimPrefix(parts[2], "a/")
	bPath := strings.TrimPrefix(parts[3], "b/")
	if aPath == bPath {
		return aPath
	}
	return bPath
}

// CheckGitRepository verifies that the current directory is a Git repository.
func CheckGitRepository(ctx context.Context) bool {
	_, err := git.PlainOpen(".")
	return err == nil
}

// GetGitDiff returns the diff of staged changes.
func GetGitDiff(ctx context.Context) (string, error) {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}
	headRef, err := repo.Head()
	if err != nil {
		return getDiffAgainstEmpty(repo)
	}
	headCommit, err := repo.CommitObject(headRef.Hash())
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD commit: %w", err)
	}
	headTree, err := headCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD tree: %w", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}
	status, err := wt.Status()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree status: %w", err)
	}
	var diffResult strings.Builder
	dmp := diffmatchpatch.New()
	for filePath, fileStatus := range status {
		if fileStatus.Staging == git.Unmodified {
			continue
		}
		newPath := filePath
		oldPath := filePath
		if fileStatus.Staging == git.Renamed && fileStatus.Extra != "" {
			oldPath = fileStatus.Extra
		}
		var oldContent string
		fileInTree, err := headTree.File(oldPath)
		if err == nil {
			reader, err := fileInTree.Blob.Reader()
			if err == nil {
				data, err := ioutil.ReadAll(reader)
				_ = reader.Close()
				if err == nil {
					oldContent = string(data)
				}
			}
		}
		var newContent string
		if fileStatus.Staging != git.Deleted {
			newContentBytes, err := ioutil.ReadFile(newPath)
			if err == nil {
				if isBinary(newContentBytes) {
					continue
				}
				newContent = string(newContentBytes)
			}
		}
		diffs := dmp.DiffMain(oldContent, newContent, true)
		patches := dmp.PatchMake(oldContent, newContent, diffs)
		patchText := dmp.PatchToText(patches)
		if strings.TrimSpace(patchText) != "" {
			diffResult.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", oldPath, newPath))
			diffResult.WriteString(patchText)
			diffResult.WriteString("\n")
		}
	}
	return diffResult.String(), nil
}

// getDiffAgainstEmpty returns a diff against an empty file for untracked files.
func getDiffAgainstEmpty(repo *git.Repository) (string, error) {
	wt, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}
	status, err := wt.Status()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree status: %w", err)
	}
	var diffResult strings.Builder
	dmp := diffmatchpatch.New()
	for filePath, fileStatus := range status {
		if fileStatus.Staging == git.Unmodified {
			continue
		}
		oldContent := ""
		var newContent string
		if fileStatus.Staging != git.Deleted {
			newContentBytes, err := ioutil.ReadFile(filePath)
			if err == nil && !isBinary(newContentBytes) {
				newContent = string(newContentBytes)
			}
		}
		diffs := dmp.DiffMain(oldContent, newContent, true)
		patches := dmp.PatchMake(oldContent, newContent, diffs)
		patchText := dmp.PatchToText(patches)
		if strings.TrimSpace(patchText) != "" {
			diffResult.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", filePath, filePath))
			diffResult.WriteString(patchText)
			diffResult.WriteString("\n")
		}
	}
	return diffResult.String(), nil
}

// AddGitmoji adds an emoji prefix to the commit message based on the commit type.
func AddGitmoji(message, commitType string) string {
	if commitType == "" {
		lowerMsg := strings.ToLower(message)
		switch {
		case strings.Contains(lowerMsg, "fix"):
			commitType = "fix"
		case strings.Contains(lowerMsg, "add"), strings.Contains(lowerMsg, "create"), strings.Contains(lowerMsg, "introduce"):
			commitType = "feat"
		case strings.Contains(lowerMsg, "doc"):
			commitType = "docs"
		case strings.Contains(lowerMsg, "refactor"):
			commitType = "refactor"
		case strings.Contains(lowerMsg, "test"):
			commitType = "test"
		case strings.Contains(lowerMsg, "perf"):
			commitType = "perf"
		case strings.Contains(lowerMsg, "build"):
			commitType = "build"
		case strings.Contains(lowerMsg, "ci"):
			commitType = "ci"
		case strings.Contains(lowerMsg, "chore"):
			commitType = "chore"
		}
	}
	if commitType == "" {
		return message
	}
	gitmojis := map[string]string{
		"feat":     "✨",
		"fix":      "🐛",
		"docs":     "📚",
		"style":    "💎",
		"refactor": "♻️",
		"test":     "🧪",
		"chore":    "🔧",
		"perf":     "🚀",
		"build":    "📦",
		"ci":       "👷",
	}
	prefix := commitType
	if emoji, ok := gitmojis[commitType]; ok {
		prefix = fmt.Sprintf("%s %s", emoji, commitType)
	}
	emojiPattern := committypes.BuildRegexPatternWithEmoji()
	if matches := emojiPattern.FindStringSubmatch(message); len(matches) > 0 {
		cleanMsg := emojiPattern.ReplaceAllString(message, "")
		return fmt.Sprintf("%s: %s", prefix, strings.TrimSpace(cleanMsg))
	}
	return fmt.Sprintf("%s: %s", prefix, message)
}

// GetHeadCommitMessage returns the commit message of HEAD.
func GetHeadCommitMessage(ctx context.Context) (string, error) {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}
	headRef, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD reference: %w", err)
	}
	commit, err := repo.CommitObject(headRef.Hash())
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD commit: %w", err)
	}
	return strings.TrimSpace(commit.Message), nil
}

// GetCurrentBranch returns the name of the current branch.
func GetCurrentBranch(ctx context.Context) (string, error) {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}
	headRef, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD reference: %w", err)
	}
	return headRef.Name().Short(), nil
}

// FilterLockFiles removes lines for lock files from the diff.
func FilterLockFiles(diff string, lockFiles []string) string {
	lines := strings.Split(diff, "\n")
	var filtered []string
	isLockFile := false
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") {
			matchesLockFile := false
			for _, lf := range lockFiles {
				p := regexp.MustCompile(`^diff --git a/(.*/)?(` + lf + `)`)
				if p.MatchString(line) {
					matchesLockFile = true
					break
				}
			}
			if matchesLockFile {
				isLockFile = true
				continue
			} else {
				isLockFile = false
			}
		}
		if !isLockFile {
			filtered = append(filtered, line)
		}
	}
	return strings.Join(filtered, "\n")
}

// CommitChanges creates a commit with the provided message using the configured commit author.
func CommitChanges(ctx context.Context, commitMessage string) error {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}
	// Use the global commit author variables set from config
	_, err = wt.Commit(commitMessage, &git.CommitOptions{
		Author: &object.Signature{
			Name:  CommitAuthorName,
			Email: CommitAuthorEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}
	return nil
}
