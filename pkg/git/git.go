package git

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/renatogalera/ai-commit/pkg/committypes"
	"github.com/renatogalera/ai-commit/pkg/config"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// lineDiff is an internal intermediate representation used by removeMovedBlocks.
type lineDiff struct {
	Op   diffmatchpatch.Operation
	Text string
}

// IsGitRepository returns true if "." is a Git repo.
func IsGitRepository(ctx context.Context) bool {
	_, err := gogit.PlainOpen(".")
	return err == nil
}

// GetGitDiffIgnoringMoves builds a textual diff based on HEAD vs current working tree,
// focused on staged changes (status.Staging != Unmodified). It removes moves and
// attempts to drop pure comment-only changes to produce a cleaner prompt for LLMs.
//
// NOTE: New content is read from the working tree, not the index. This is a known limitation
// if the user stages partial changes and then edits further. To make it *exactly* reflect the
// index, you’d need to read blobs from the index (or shell-out to `git show :path`).
func GetGitDiffIgnoringMoves(ctx context.Context) (string, error) {
	repo, err := gogit.PlainOpen(".")
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
	if status.IsClean() {
		return "", nil
	}

	dmp := diffmatchpatch.New()
	var diffResult strings.Builder

	headRef, err := repo.Head()
	if err != nil {
		// No HEAD (e.g., first commit) – treat as diff against empty tree.
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
		if fileStatus.Staging == gogit.Unmodified {
			continue
		}

		oldPath, newPath := filePath, filePath
		if fileStatus.Staging == gogit.Renamed && fileStatus.Extra != "" {
			oldPath = fileStatus.Extra
		}

		var oldContent string
		if fileInTree, err := headTree.File(oldPath); err == nil {
			if reader, err := fileInTree.Blob.Reader(); err == nil {
				data, _ := io.ReadAll(reader)
				_ = reader.Close()
				oldContent = string(data)
			}
		}

		var newContent string
		if fileStatus.Staging != gogit.Deleted {
			// NOTE: reads working tree; for exact staged content, use index blob or `git show :path`.
			if data, err := os.ReadFile(newPath); err == nil && !isBinary(data) {
				newContent = string(data)
			}
		}

		// Skip binary/no-content situations.
		if oldContent == "" && newContent == "" {
			continue
		}

		// Build diff, clean up, and remove simple moved blocks.
		diffs := dmp.DiffMain(oldContent, newContent, true)
		diffs = dmp.DiffCleanupSemantic(diffs)
		diffs = removeMovedBlocks(diffs)

		if len(diffs) == 0 {
			continue
		}

		// IMPORTANT: Correct usage. Build patches from the *two texts*.
		patches := dmp.PatchMake(oldContent, newContent)
		patchText := dmp.PatchToText(patches)
		if strings.TrimSpace(patchText) == "" {
			continue
		}

		// Prepend a path header to aid parsing later.
		diffResult.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", oldPath, newPath))
		diffResult.WriteString(patchText)
		diffResult.WriteString("\n")
	}

	diff := diffResult.String()
	cleanedDiff := cleanupDiff(diff)
	if strings.TrimSpace(cleanedDiff) == "" {
		return "", nil
	}
	return cleanedDiff, nil
}

// getDiffAgainstEmptyIgnoringMoves computes a diff vs empty repo.
func getDiffAgainstEmptyIgnoringMoves(repo *gogit.Repository) (string, error) {
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
		if fileStatus.Staging == gogit.Unmodified {
			continue
		}
		var newContent string
		if fileStatus.Staging != gogit.Deleted {
			data, err := os.ReadFile(filePath)
			if err == nil && !isBinary(data) {
				newContent = string(data)
			}
		}
		diffs := dmp.DiffMain("", newContent, true)
		diffs = dmp.DiffCleanupSemantic(diffs)
		diffs = removeMovedBlocks(diffs)

		patches := dmp.PatchMake("", newContent) // Correct two-arg variant
		patchText := dmp.PatchToText(patches)
		if strings.TrimSpace(patchText) == "" {
			continue
		}
		diffResult.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", filePath, filePath))
		diffResult.WriteString(patchText)
		diffResult.WriteString("\n")
	}
	return diffResult.String(), nil
}

// removeMovedBlocks naively removes added lines that exactly match previously deleted lines.
// It’s line-based; duplicates are decremented from a multiset to avoid over-deleting.
func removeMovedBlocks(diffs []diffmatchpatch.Diff) []diffmatchpatch.Diff {
	deleteMap := make(map[string]int)
	var finalList []lineDiff

	for _, df := range diffs {
		if df.Type == diffmatchpatch.DiffDelete {
			for _, ln := range strings.Split(df.Text, "\n") {
				t := strings.TrimSpace(ln)
				if t != "" {
					deleteMap[t]++
				}
			}
		}
	}

	for _, df := range diffs {
		switch df.Type {
		case diffmatchpatch.DiffInsert:
			for _, ln := range strings.Split(df.Text, "\n") {
				t := strings.TrimSpace(ln)
				if t == "" {
					finalList = append(finalList, lineDiff{Op: df.Type, Text: ln})
					continue
				}
				if deleteMap[t] > 0 {
					deleteMap[t]--
					continue // treat as moved
				}
				finalList = append(finalList, lineDiff{Op: df.Type, Text: ln})
			}
		case diffmatchpatch.DiffEqual:
			for _, ln := range strings.Split(df.Text, "\n") {
				finalList = append(finalList, lineDiff{Op: df.Type, Text: ln})
			}
		case diffmatchpatch.DiffDelete:
			for _, ln := range strings.Split(df.Text, "\n") {
				finalList = append(finalList, lineDiff{Op: df.Type, Text: ln})
			}
		}
	}

	return reassembleLineDiffs(finalList)
}

// reassembleLineDiffs compresses adjacent ops back into standard Diff chunks.
func reassembleLineDiffs(lines []lineDiff) []diffmatchpatch.Diff {
	if len(lines) == 0 {
		return nil
	}
	var out []diffmatchpatch.Diff
	currentOp := lines[0].Op
	var buf bytes.Buffer

	flush := func() {
		if buf.Len() == 0 {
			return
		}
		out = append(out, diffmatchpatch.Diff{
			Type: currentOp,
			Text: strings.TrimSuffix(buf.String(), "\n"),
		})
		buf.Reset()
	}

	for _, ld := range lines {
		if ld.Op != currentOp {
			flush()
			currentOp = ld.Op
		}
		buf.WriteString(ld.Text)
		buf.WriteByte('\n')
	}
	flush()
	return out
}

// isBinary uses net/http content detection to skip media/archives/fonts/etc.
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

// FilterLockFiles drops entire file sections that match any of the provided lock file names.
func FilterLockFiles(diff string, lockFiles []string) string {
	if len(lockFiles) == 0 {
		return diff
	}
	lines := strings.Split(diff, "\n")
	var filtered []string
	isLockFile := false

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			matchFound := false
			for _, lf := range lockFiles {
				pattern := fmt.Sprintf(`^diff --git a/(.*/)?(%s)$`, regexp.QuoteMeta(lf))
				matched, _ := regexp.MatchString(pattern, strings.TrimSpace(line))
				if matched {
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

// CommitChanges creates a commit with a supplied message and the configured author identity.
func CommitChanges(ctx context.Context, commitMessage string) error {
	repo, err := gogit.PlainOpen(".")
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}
	_, err = worktree.Commit(commitMessage, &gogit.CommitOptions{
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

// GetHeadCommitMessage returns the HEAD commit message.
func GetHeadCommitMessage(ctx context.Context) (string, error) {
	repo, err := gogit.PlainOpen(".")
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

// GetCurrentBranch returns the short name of the current branch.
func GetCurrentBranch(ctx context.Context) (string, error) {
	repo, err := gogit.PlainOpen(".")
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}
	headRef, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD reference: %w", err)
	}
	return headRef.Name().Short(), nil
}

// PrependCommitType ensures there's a single prefix (optionally with gitmoji) and prepends it.
func PrependCommitType(message, commitType string, withEmoji bool) string {
	if commitType == "" {
		return message
	}
	regex := committypes.BuildRegexPatternWithEmoji()
	message = regex.ReplaceAllString(message, "")
	message = strings.TrimSpace(message)
	if withEmoji {
		return AddGitmoji(message, commitType)
	}
	return fmt.Sprintf("%s: %s", commitType, message)
}

// AddGitmoji adds emoji if configured, or just ensures a clean type prefix.
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
	if emojiPattern.MatchString(message) {
		message = emojiPattern.ReplaceAllString(message, "")
	}
	return fmt.Sprintf("%s: %s", prefix, strings.TrimSpace(message))
}

// DiffChunk represents a parsed @@ hunk from a diff.
type DiffChunk struct {
	FilePath   string
	HunkHeader string
	Lines      []string
}

// ParseDiffToChunks splits our diff into per-file hunk chunks used by the interactive splitter.
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

// parseFilePath extracts the canonical file path from a "diff --git a/X b/Y" header.
func parseFilePath(diffLine string) string {
	parts := strings.Fields(diffLine)
	// Expected: ["diff","--git","a/<path>","b/<path>"]
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

// cleanupDiff removes comment-only changes and simple "move" no-ops from DMP patches.
func cleanupDiff(diff string) string {
	lines := strings.Split(diff, "\n")
	var cleaned []string
	skipContext := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Keep hunk headers intact.
		if strings.HasPrefix(line, "@@") {
			skipContext = false
			cleaned = append(cleaned, line)
			continue
		}

		// Drop pure-context lines if we're skipping a comment-only section.
		if skipContext && strings.HasPrefix(line, " ") {
			continue
		}

		if isCommentOnlyChange(line) || isPureMovement(lines, i) {
			skipContext = true
			continue
		}

		cleaned = append(cleaned, line)
		skipContext = false
	}
	return strings.Join(cleaned, "\n")
}

// isCommentOnlyChange detects when a diff line (+/-) only changes comments.
func isCommentOnlyChange(line string) bool {
	line = strings.TrimRight(line, "\r\n")

	if len(line) == 0 {
		return false
	}

	// Only consider +/- lines as candidate changes.
	first := line[0]
	if first != '+' && first != '-' {
		return false
	}

	// Strip the +/- and any leading whitespace before checking comment markers.
	payload := strings.TrimSpace(line[1:])

	// Common comment starters across languages.
	commentPattern := regexp.MustCompile(`^(//|/\*|\*|#|--|<!--|;|"""|'|\(\*).*`)
	return commentPattern.MatchString(payload)
}

// isPureMovement returns true if a '-' line is immediately followed by an identical '+' line.
func isPureMovement(lines []string, i int) bool {
	if i >= len(lines)-1 {
		return false
	}
	cur := strings.TrimSpace(lines[i])
	next := strings.TrimSpace(lines[i+1])
	if strings.HasPrefix(cur, "-") && strings.HasPrefix(next, "+") {
		removed := strings.TrimSpace(strings.TrimPrefix(cur, "-"))
		added := strings.TrimSpace(strings.TrimPrefix(next, "+"))
		return removed != "" && removed == added
	}
	return false
}

// buildPatch is used by the splitter to apply selected hunks to the index.
func buildPatch(chunks []DiffChunk, selected map[int]bool) (string, error) {
	var sb strings.Builder
	for i, c := range chunks {
		if !selected[i] {
			continue
		}
		// Minimal unified-diff header + hunks. This is enough for `git apply --cached`.
		sb.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", c.FilePath, c.FilePath))
		sb.WriteString("--- a/" + c.FilePath + "\n")
		sb.WriteString("+++ b/" + c.FilePath + "\n")
		sb.WriteString(c.HunkHeader + "\n")
		for _, line := range c.Lines {
			sb.WriteString(line + "\n")
		}
	}
	return sb.String(), nil
}

// partialCommit applies a synthesized patch to the index and commits with an AI-generated message.
func partialCommit(chunks []DiffChunk, selected map[int]bool, client any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	patch, err := buildPatch(chunks, selected)
	if err != nil {
		return err
	}
	if strings.TrimSpace(patch) == "" {
		return fmt.Errorf("no chunks selected")
	}

	cmd := exec.CommandContext(ctx, "git", "apply", "--cached", "-")
	cmd.Stdin = strings.NewReader(patch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to apply patch: %w", err)
	}

	// Recompute diff (still from working tree, see note above).
	partialDiff, err := GetGitDiffIgnoringMoves(ctx)
	if err != nil {
		return fmt.Errorf("failed to get partial diff: %w", err)
	}

	// We accept a loose `any` to avoid import cycles. The caller passes ai.AIClient.
	type aiClient interface {
		GetCommitMessage(ctx context.Context, prompt string) (string, error)
	}
	ac, ok := client.(aiClient)
	if !ok {
		return fmt.Errorf("invalid AI client")
	}

	prompt := fmt.Sprintf(`Generate a commit message for the following partial diff.
The message must follow Conventional Commits style.
Output only the commit message.

Diff:
%s
`, partialDiff)

	msg, err := ac.GetCommitMessage(ctx, prompt)
	if err != nil {
		return fmt.Errorf("AI error: %w", err)
	}
	if err := CommitChanges(ctx, strings.TrimSpace(msg)); err != nil {
		return err
	}
	return nil
}
