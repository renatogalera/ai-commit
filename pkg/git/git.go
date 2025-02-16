package git

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/renatogalera/ai-commit/pkg/committypes"
	"github.com/renatogalera/ai-commit/pkg/config"
	"github.com/sergi/go-diff/diffmatchpatch"
)

func IsGitRepository(ctx context.Context) bool {
	_, err := git.PlainOpen(".")
	return err == nil
}

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
			newContentBytes, err := os.ReadFile(newPath)
			if err == nil && !isBinary(newContentBytes) {
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
			data, err := os.ReadFile(filePath)
			if err == nil && !isBinary(data) {
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

func isBinary(data []byte) bool {
	contentType := http.DetectContentType(data)
	return strings.HasPrefix(contentType, "image/") ||
		strings.HasPrefix(contentType, "video/") ||
		strings.HasPrefix(contentType, "audio/") ||
		contentType == "application/octet-stream" ||
		contentType == "application/pdf" ||
		contentType == "application/zip" ||
		strings.Contains(contentType, "font")
}

// PrependCommitType checks if we want to add an emoji (based on config) and/or commitType:
// e.g., "fix: some text" or "🐛 fix: some text".
func PrependCommitType(message, commitType string, withEmoji bool) string {
	if commitType == "" {
		return message
	}
	// Remove any existing commit type prefix (with or without emoji).
	regex := committypes.BuildRegexPatternWithEmoji()
	message = regex.ReplaceAllString(message, "")
	message = strings.TrimSpace(message)

	if withEmoji {
		return AddGitmoji(message, commitType)
	}
	return fmt.Sprintf("%s: %s", commitType, message)
}

func AddGitmoji(message, commitType string) string {
	if commitType == "" {
		return message
	}
	emoji := committypes.GetEmojiForType(commitType)
	prefix := commitType
	if emoji != "" {
		prefix = fmt.Sprintf("%s %s", emoji, commitType)
	}
	// If there's already a commit type prefix, remove it
	emojiPattern := committypes.BuildRegexPatternWithEmoji()
	if matches := emojiPattern.FindStringSubmatch(message); len(matches) > 0 {
		message = emojiPattern.ReplaceAllString(message, "")
	}
	return fmt.Sprintf("%s: %s", prefix, strings.TrimSpace(message))
}

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

func FilterLockFiles(diff string, lockFiles []string) string {
	if len(lockFiles) == 0 {
		return diff
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
			Name:  config.DefaultAuthorName,
			Email: config.DefaultAuthorEmail,
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
