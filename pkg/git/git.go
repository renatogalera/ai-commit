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
	"github.com/sergi/go-diff/diffmatchpatch"
)

// DiffChunk represents a single "hunk" within a diff for a particular file.
type DiffChunk struct {
	FilePath   string
	HunkHeader string
	Lines      []string
}

// isBinary checks if the provided data is binary by scanning for a null byte.
// This is a simple heuristic that works in many cases.
func isBinary(data []byte) bool {
	// An empty file is not considered binary.
	if len(data) == 0 {
		return false
	}
	// If a null byte is found, consider it binary.
	return bytes.IndexByte(data, 0) != -1
}

// ParseDiffToChunks splits a unified diff into a list of DiffChunk structs.
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

// parseFilePath attempts to parse the file path from a "diff --git" line.
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

// CheckGitRepository verifies if the current folder is a Git repository using go-git.
func CheckGitRepository(ctx context.Context) bool {
	_, err := git.PlainOpen(".")
	return err == nil
}

// GetGitDiff returns a unified diff of staged changes by comparing the HEAD tree and the working directory.
// This version skips diffing files that are detected as binary.
func GetGitDiff(ctx context.Context) (string, error) {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

	headRef, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD reference: %w", err)
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
		// Only process files that have staged changes.
		if fileStatus.Staging == git.Unmodified {
			continue
		}

		// Read the old content (from HEAD) if available.
		var oldContent string
		fileInTree, err := headTree.File(filePath)
		if err == nil {
			reader, err := fileInTree.Blob.Reader()
			if err == nil {
				data, err := ioutil.ReadAll(reader)
				reader.Close()
				if err == nil {
					oldContent = string(data)
				}
			}
		}

		// Read the new content from the file system.
		newContentBytes, err := ioutil.ReadFile(filePath)
		var newContent string
		if err == nil {
			// Skip binary files based on the new file content.
			if isBinary(newContentBytes) {
				// Optionally, you can print a message or log that a binary file was skipped.
				continue
			}
			newContent = string(newContentBytes)
		} else {
			newContent = ""
		}

		// Generate a unified diff using diffmatchpatch.
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

// GetHeadCommitMessage retrieves the last commit message on HEAD using go-git.
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

// GetCurrentBranch returns the current Git branch name using go-git.
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

// FilterLockFiles removes diff sections of lock files from the analysis.
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

// CommitChanges creates a commit with the provided message using go-git.
func CommitChanges(ctx context.Context, commitMessage string) error {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}
	_, err = wt.Commit(commitMessage, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "ai-commit",
			Email: "rennato@gmail.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}
	return nil
}
