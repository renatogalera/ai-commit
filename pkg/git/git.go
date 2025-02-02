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

func runCommandContext(ctx context.Context, cmdName string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, cmdName, args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to run %s: %w", cmdName, err)
	}
	return string(output), nil
}

func runCommand(cmdName string, args ...string) (string, error) {
	return runCommandContext(context.Background(), cmdName, args...)
}

func CheckGitRepository() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := runCommandContext(ctx, "git", "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) == "true"
}

func GetGitDiff() (string, error) {
	return runCommand("git", "diff", "--staged")
}

func GetHeadCommitMessage() (string, error) {
	out, err := runCommand("git", "log", "-1", "--pretty=%B")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func GetCurrentBranch() (string, error) {
	out, err := runCommand("git", "branch", "--show-current")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func FilterLockFiles(diff string, lockFiles []string) string {
	lines := strings.Split(diff, "\n")
	var filtered []string
	isLockFile := false

	patterns := make([]*regexp.Regexp, 0, len(lockFiles))
	for _, lf := range lockFiles {
		reg := regexp.MustCompile(`^diff --git a/(.*/)?(` + lf + `)`)
		patterns = append(patterns, reg)
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") {
			matchesLockFile := false
			for _, p := range patterns {
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

func CommitChanges(commitMessage string) error {
	cmd := exec.Command("git", "commit", "-F", "-")
	cmd.Stdin = strings.NewReader(commitMessage)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}
	return nil
}
