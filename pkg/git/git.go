package git

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http" // Import net/http for DetectContentType
	"regexp"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/renatogalera/ai-commit/pkg/committypes"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// Global variables for commit author details.
var (
	CommitAuthorName  = "ai-commit"
	CommitAuthorEmail = "ai-commit@example.com"
)

// IsGitRepository checks if the current directory is a Git repository.
func IsGitRepository(ctx context.Context) bool {
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
	worktree, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}
	status, err := worktree.Status()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree status: %w", err)
	}

	var diffResult strings.Builder
	dmp := diffmatchpatch.New()
	for filePath, fileStatus := range status {
		if fileStatus.Staging == git.Unmodified {
			continue
		}
		oldPath, newPath := filePath, filePath
		if fileStatus.Staging == git.Renamed && fileStatus.Extra != "" {
			oldPath = fileStatus.Extra
		}
		var oldContent string
		if fileInTree, err := headTree.File(oldPath); err == nil {
			reader, err := fileInTree.Blob.Reader()
			if err == nil {
				data, err := io.ReadAll(reader)
				reader.Close()
				if err == nil {
					oldContent = string(data)
				}
			}
		}
		var newContent string
		if fileStatus.Staging != git.Deleted {
			newContentBytes, err := ioutil.ReadFile(newPath)
			if err == nil && !isBinary(newContentBytes) { // Use better binary check with net/http
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

// getDiffAgainstEmpty generates a diff for untracked files.
func getDiffAgainstEmpty(repo *git.Repository) (string, error) {
	worktree, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}
	status, err := worktree.Status()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree status: %w", err)
	}
	var diffResult strings.Builder
	dmp := diffmatchpatch.New()
	for filePath, fileStatus := range status {
		if fileStatus.Staging == git.Unmodified {
			continue
		}
		var newContent string
		if fileStatus.Staging != git.Deleted {
			data, err := ioutil.ReadFile(filePath)
			if err == nil && !isBinary(data) { // Use better binary check with net/http
				newContent = string(data)
			}
		}
		diffs := dmp.DiffMain("", newContent, true)
		patches := dmp.PatchMake("", newContent, diffs)
		patchText := dmp.PatchToText(patches)
		if strings.TrimSpace(patchText) != "" {
			diffResult.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", filePath, filePath))
			diffResult.WriteString(patchText)
			diffResult.WriteString("\n")
		}
	}
	return diffResult.String(), nil
}

// isBinary checks if the data is likely binary using net/http.DetectContentType.
func isBinary(data []byte) bool {
	contentType := http.DetectContentType(data)
	return strings.HasPrefix(contentType, "image/") ||
		strings.HasPrefix(contentType, "video/") ||
		strings.HasPrefix(contentType, "audio/") ||
		contentType == "application/octet-stream" ||
		contentType == "application/pdf" || // Add common binary types if needed
		contentType == "application/zip" ||
		strings.Contains(contentType, "font")
}

// AddGitmoji adds an emoji based on commit type.
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
		message = emojiPattern.ReplaceAllString(message, "")
		return fmt.Sprintf("%s: %s", prefix, strings.TrimSpace(message))
	}
	return fmt.Sprintf("%s: %s", prefix, message)
}

// PrependCommitType ensures the commit type prefix is added to the message.
// If withEmoji is true, it uses AddGitmoji; otherwise, it simply prepends the type.
func PrependCommitType(message, commitType string, withEmoji bool) string {
	if commitType == "" {
		return message
	}
	// Remove any existing commit type prefix.
	regex := committypes.BuildRegexPatternWithEmoji()
	message = regex.ReplaceAllString(message, "")
	message = strings.TrimSpace(message)
	if withEmoji {
		return AddGitmoji(message, commitType)
	}
	return fmt.Sprintf("%s: %s", commitType, message)
}

// GetHeadCommitMessage returns the HEAD commit message.
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

// GetCurrentBranch returns the current Git branch name.
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

// FilterLockFiles filters out diffs for specified lock files.
func FilterLockFiles(diff string, lockFiles []string) string {
	if len(lockFiles) == 0 {
		return diff // Return original diff if no lock files specified.
	}
	lines := strings.Split(diff, "\n")
	var filtered []string
	isLockFile := false
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") {
			matchFound := false
			for _, lf := range lockFiles {
				pattern := fmt.Sprintf(`^diff --git a/(.*/)?(%s)`, lf)
				if matched, _ := regexp.MatchString(pattern, line); matched {
					matchFound = true
					break
				}
			}
			isLockFile = matchFound
			if isLockFile {
				continue
			}
		}
		if !isLockFile {
			filtered = append(filtered, line)
		}
	}
	return strings.Join(filtered, "\n")
}

// CommitChanges creates a commit with the provided message.
func CommitChanges(ctx context.Context, commitMessage string) error {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}
	_, err = worktree.Commit(commitMessage, &git.CommitOptions{
		Author: &object.Signature{
			Name:  CommitAuthorName,
			Email: CommitAuthorEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}
	return nil
}

// DiffChunk represents a section of a Git diff.
type DiffChunk struct {
	FilePath   string
	HunkHeader string
	Lines      []string
}

// ParseDiffToChunks splits a diff string into chunks.
func ParseDiffToChunks(diff string) ([]DiffChunk, error) {
	lines := strings.Split(diff, "\n")
	var chunks []DiffChunk
	var currentChunk *DiffChunk
	var currentFile string
	inHunk := false
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			if currentChunk != nil {
				chunks = append(chunks, *currentChunk)
				currentChunk = nil
			}
			currentFile = parseFilePath(line)
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

// parseFilePath extracts the file path from a diff header.
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
