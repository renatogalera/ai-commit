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

// lineDiff is our small struct for line-by-line expansions
// (needed by removeMovedBlocks and reassembleLineDiffs).
type lineDiff struct {
	Op   diffmatchpatch.Operation
	Text string
}

// IsGitRepository checks if we're in a valid git repo.
func IsGitRepository(ctx context.Context) bool {
	_, err := git.PlainOpen(".")
	return err == nil
}

// GetGitDiffIgnoringMoves is like the old GetGitDiff but also
func GetGitDiffIgnoringMoves(ctx context.Context) (string, error) {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree status: %w", err)
	}

	// Early return if no changes
	if status.IsClean() {
		return "", nil
	}

	dmp := diffmatchpatch.New()
	var diffResult strings.Builder

	headRef, err := repo.Head()
	if err != nil {
		// Handle new repos without HEAD
		return getDiffAgainstEmptyIgnoringMoves(repo)
	}

	headCommit, err := repo.CommitObject(headRef.Hash())
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD commit: %w", err)
	}

	headTree, err := headCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD tree: %w", err)
	}

	for filePath, fileStatus := range status {
		if fileStatus.Staging == git.Unmodified {
			continue
		}

		oldPath, newPath := filePath, filePath
		if fileStatus.Staging == git.Renamed && fileStatus.Extra != "" {
			oldPath = fileStatus.Extra
		}

		// Get old content
		var oldContent string
		if fileInTree, err := headTree.File(oldPath); err == nil {
			if reader, err := fileInTree.Blob.Reader(); err == nil {
				data, _ := io.ReadAll(reader)
				reader.Close()
				oldContent = string(data)
			}
		}

		// Get new content
		var newContent string
		if fileStatus.Staging != git.Deleted {
			if data, err := os.ReadFile(newPath); err == nil && !isBinary(data) {
				newContent = string(data)
			}
		}

		// Skip if both contents are empty
		if oldContent == "" && newContent == "" {
			continue
		}

		diffs := dmp.DiffMain(oldContent, newContent, true)
		diffs = dmp.DiffCleanupSemantic(diffs)
		diffs = removeMovedBlocks(diffs)

		// Only create patch if there are actual differences
		if len(diffs) > 0 {
			patches := dmp.PatchMake(oldContent, diffs)
			patchText := dmp.PatchToText(patches)
			if strings.TrimSpace(patchText) != "" {
				diffResult.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", oldPath, newPath))
				diffResult.WriteString(patchText)
				diffResult.WriteString("\n")
			}
		}
	}

	diff := diffResult.String()
	cleanedDiff := cleanupDiff(diff)

	if strings.TrimSpace(cleanedDiff) == "" {
		return "", nil
	}

	return cleanedDiff, nil

}

func getDiffAgainstEmptyIgnoringMoves(repo *git.Repository) (string, error) {
	worktree, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}
	status, err := worktree.Status()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree status: %w", err)
	}

	dmp := diffmatchpatch.New()
	var diffResult strings.Builder

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
		diffs = dmp.DiffCleanupSemantic(diffs)

		diffs = removeMovedBlocks(diffs)

		patches := dmp.PatchMake("", "", diffs)
		patchText := dmp.PatchToText(patches)
		if strings.TrimSpace(patchText) != "" {
			diffResult.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", filePath, filePath))
			diffResult.WriteString(patchText)
			diffResult.WriteString("\n")
		}
	}
	return diffResult.String(), nil
}

// removeMovedBlocks attempts to detect lines that are purely moved from one location
// to another (delete+insert of the same line) and remove them from the final diff.
func removeMovedBlocks(diffs []diffmatchpatch.Diff) []diffmatchpatch.Diff {
	// Expand multi-line text blocks into lineDiff entries.
	var expanded []lineDiff
	for _, df := range diffs {
		lines := strings.Split(df.Text, "\n")
		// if the last line is empty from a trailing newline, remove
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		for _, ln := range lines {
			expanded = append(expanded, lineDiff{
				Op:   df.Type,
				Text: ln,
			})
		}
	}

	// We'll track how many times each line was "DiffDelete" in deleteMap
	deleteMap := make(map[string]int)
	var finalList []lineDiff

	// First pass: whenever we see a DiffInsert that matches a line from deleteMap, we remove it (i.e. treat as moved).
	for _, e := range expanded {
		switch e.Op {
		case diffmatchpatch.DiffDelete:
			deleteMap[e.Text]++
		case diffmatchpatch.DiffInsert:
			if deleteMap[e.Text] > 0 {
				// matched a moved line
				deleteMap[e.Text]--
			} else {
				// real inserted line
				finalList = append(finalList, e)
			}
		case diffmatchpatch.DiffEqual:
			// keep equal lines
			finalList = append(finalList, e)
		}
	}

	// Now we might still have leftover lines in deleteMap that never found an Insert match.
	// We'll re-inject them in the correct place (before the next DiffEqual or at the end).
	results := []lineDiff{}
	deleteMapCopy := make(map[string]int)
	for k, v := range deleteMap {
		deleteMapCopy[k] = v
	}

	for _, e := range finalList {
		if e.Op == diffmatchpatch.DiffEqual {
			// flush leftover unmatched deletes
			for lineText, count := range deleteMapCopy {
				for i := 0; i < count; i++ {
					results = append(results, lineDiff{Op: diffmatchpatch.DiffDelete, Text: lineText})
				}
				delete(deleteMapCopy, lineText)
			}
		}
		results = append(results, e)
	}
	// flush any leftover deletes at the very end
	for lineText, count := range deleteMapCopy {
		for i := 0; i < count; i++ {
			results = append(results, lineDiff{Op: diffmatchpatch.DiffDelete, Text: lineText})
		}
	}

	// Reassemble them back into bigger Diff objects
	return reassembleLineDiffs(results)
}

// reassembleLineDiffs merges consecutive lineDiffs of the same Op back into diffmatchpatch.Diff
func reassembleLineDiffs(lines []lineDiff) []diffmatchpatch.Diff {
	if len(lines) == 0 {
		return nil
	}
	var out []diffmatchpatch.Diff
	currentOp := lines[0].Op
	var chunkLines []string

	flush := func() {
		if len(chunkLines) > 0 {
			out = append(out, diffmatchpatch.Diff{
				Type: currentOp,
				Text: strings.Join(chunkLines, "\n") + "\n",
			})
		}
	}

	for _, ld := range lines {
		if ld.Op != currentOp {
			flush()
			currentOp = ld.Op
			chunkLines = []string{ld.Text}
		} else {
			chunkLines = append(chunkLines, ld.Text)
		}
	}
	flush()
	return out
}

// isBinary checks if data is recognized as a binary file (image, pdf, font, etc.)
func isBinary(data []byte) bool {
	contentType := http.DetectContentType(data)
	if strings.HasPrefix(contentType, "image/") ||
		strings.HasPrefix(contentType, "video/") ||
		strings.HasPrefix(contentType, "audio/") ||
		contentType == "application/octet-stream" ||
		contentType == "application/pdf" ||
		contentType == "application/zip" ||
		strings.Contains(contentType, "font") {
		return true
	}
	return false
}

// FilterLockFiles removes any lockfiles from the diff (like go.sum, yarn.lock, etc.).
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

// CommitChanges creates a commit with the specified message.
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

// GetHeadCommitMessage returns HEAD commit message text.
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

// GetCurrentBranch returns the currently checked-out branch name.
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

// PrependCommitType prepends "type: " (and possibly an emoji) to the commit message.
func PrependCommitType(message, commitType string, withEmoji bool) string {
	if commitType == "" {
		return message
	}
	// remove any existing "feat: " or "🐛 fix:" prefix
	regex := committypes.BuildRegexPatternWithEmoji()
	message = regex.ReplaceAllString(message, "")
	message = strings.TrimSpace(message)

	if withEmoji {
		return AddGitmoji(message, commitType)
	}
	return fmt.Sprintf("%s: %s", commitType, message)
}

// AddGitmoji checks if there's a known emoji for the type.
func AddGitmoji(message, commitType string) string {
	if commitType == "" {
		return message
	}
	emoji := committypes.GetEmojiForType(commitType)
	prefix := commitType
	if emoji != "" {
		prefix = fmt.Sprintf("%s %s", emoji, commitType)
	}
	emojiPattern := committypes.BuildRegexPatternWithEmoji()
	if matches := emojiPattern.FindStringSubmatch(message); len(matches) > 0 {
		message = emojiPattern.ReplaceAllString(message, "")
	}
	return fmt.Sprintf("%s: %s", prefix, strings.TrimSpace(message))
}

// DiffChunk + ParseDiffToChunks remain the same for interactive-split usage:
type DiffChunk struct {
	FilePath   string
	HunkHeader string
	Lines      []string
}

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

// Add these new filtering functions
func cleanupDiff(diff string) string {
	lines := strings.Split(diff, "\n")
	var cleaned []string
	skipNext := false

	for i, line := range lines {
		// Skip empty lines or pure whitespace changes
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Skip comment-only changes
		if isCommentChange(line) {
			continue
		}

		// Skip pure movement indicators
		if isPureMovement(lines, i) {
			skipNext = true
			continue
		}

		if skipNext {
			skipNext = false
			continue
		}

		cleaned = append(cleaned, line)
	}

	return strings.Join(cleaned, "\n")
}

func isCommentChange(line string) bool {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "+//") || strings.HasPrefix(line, "-//") {
		return true
	}
	if strings.HasPrefix(line, "+/*") || strings.HasPrefix(line, "-/*") {
		return true
	}
	if strings.HasPrefix(line, "+*") || strings.HasPrefix(line, "-*") {
		return true
	}
	return false
}

func isPureMovement(lines []string, currentIndex int) bool {
	if currentIndex >= len(lines)-1 {
		return false
	}

	current := strings.TrimSpace(lines[currentIndex])
	next := strings.TrimSpace(lines[currentIndex+1])

	// Check if it's a remove followed by add of same content
	if strings.HasPrefix(current, "-") && strings.HasPrefix(next, "+") {
		removedContent := strings.TrimPrefix(current, "-")
		addedContent := strings.TrimPrefix(next, "+")
		return removedContent == addedContent
	}

	return false
}
