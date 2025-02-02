package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// defaultGitTimeout is a smaller Git command timeout used if the caller
// does not specify or passes nil context. You can adjust as needed.
var defaultGitTimeout = 15 * time.Second

// runCommandContext runs a shell command with the provided context.
func runCommandContext(ctx context.Context, cmdName string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, cmdName, args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to run %s: %w", cmdName, err)
	}
	return string(output), nil
}

// CheckGitRepository verifies if the current folder is inside a Git repository.
func CheckGitRepository(ctx context.Context) bool {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), defaultGitTimeout)
		defer cancel()
	}
	out, err := runCommandContext(ctx, "git", "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) == "true"
}

// GetGitDiff returns the staged diff as a string.
func GetGitDiff(ctx context.Context) (string, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), defaultGitTimeout)
		defer cancel()
	}
	return runCommandContext(ctx, "git", "diff", "--staged")
}

// GetHeadCommitMessage retrieves the last commit message on HEAD.
func GetHeadCommitMessage(ctx context.Context) (string, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), defaultGitTimeout)
		defer cancel()
	}
	out, err := runCommandContext(ctx, "git", "log", "-1", "--pretty=%B")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// GetCurrentBranch returns the current Git branch name.
func GetCurrentBranch(ctx context.Context) (string, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), defaultGitTimeout)
		defer cancel()
	}
	out, err := runCommandContext(ctx, "git", "branch", "--show-current")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// FilterLockFiles removes diff sections belonging to lock files from the analysis.
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

// CommitChanges takes a commit message and creates a commit with it.
func CommitChanges(ctx context.Context, commitMessage string) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), defaultGitTimeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, "git", "commit", "-F", "-")
	cmd.Stdin = strings.NewReader(commitMessage)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}
	return nil
}
