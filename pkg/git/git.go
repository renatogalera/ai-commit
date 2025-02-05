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

type DiffChunk struct {
	FilePath   string
	HunkHeader string
	Lines      []string
}

func isBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	return bytes.IndexByte(data, 0) != -1
}

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

func CheckGitRepository(ctx context.Context) bool {
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
