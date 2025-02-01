package git

import (
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

func CheckGitRepository() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

func GetGitDiff() (string, error) {
	cmd := exec.Command("git", "diff", "--staged")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func GetCurrentBranch() (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func FilterLockFiles(diff string) string {
	lines := strings.Split(diff, "\n")
	var filtered []string
	isLockFile := false
	regex := regexp.MustCompile(`^diff --git a/(.*/)?(go\.mod|go\.sum)`)
	for _, line := range lines {
		if regex.MatchString(line) {
			isLockFile = true
			continue
		}
		if isLockFile && strings.HasPrefix(line, "diff --git") {
			isLockFile = false
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
		return errors.New("failed to commit changes: " + err.Error())
	}
	return nil
}
