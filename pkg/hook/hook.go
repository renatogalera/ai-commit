package hook

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const hookName = "prepare-commit-msg"

// hookMarker is embedded in the generated script for identification.
const hookMarker = "# ai-commit-managed-hook"

// HooksDir returns the path to the git hooks directory for the current repo.
func HooksDir() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}
	gitDir := strings.TrimSpace(string(out))
	return filepath.Join(gitDir, "hooks"), nil
}

// HookPath returns the full path to the prepare-commit-msg hook file.
func HookPath() (string, error) {
	dir, err := HooksDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, hookName), nil
}

// IsInstalled checks whether an ai-commit-managed hook exists.
func IsInstalled() (bool, error) {
	path, err := HookPath()
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, nil // file doesn't exist
	}
	return strings.Contains(string(data), hookMarker), nil
}

// ExistingHookIsThirdParty checks if a hook file exists that was NOT
// installed by ai-commit.
func ExistingHookIsThirdParty() (bool, error) {
	path, err := HookPath()
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, nil // no hook file
	}
	return !strings.Contains(string(data), hookMarker), nil
}

// binaryName returns the name to use in the hook script.
// Uses os.Executable when possible, falling back to "ai-commit".
func binaryName() string {
	exe, err := os.Executable()
	if err != nil {
		return "ai-commit"
	}
	// If running via go run (temp dir), use the plain binary name
	if strings.Contains(exe, "go-build") || strings.Contains(exe, os.TempDir()) {
		return "ai-commit"
	}
	return exe
}

// HookScript returns the shell script content for the prepare-commit-msg hook.
func HookScript() string {
	bin := binaryName()
	return fmt.Sprintf(`#!/bin/sh
%s
# Installed by ai-commit. Do not edit manually.

COMMIT_MSG_FILE=$1
COMMIT_SOURCE=$2

# Only generate for normal commits (not merge, squash, amend, or -m flag)
if [ -z "$COMMIT_SOURCE" ]; then
    MSG=$(%s --msg-only 2>/dev/null)
    if [ $? -eq 0 ] && [ -n "$MSG" ]; then
        printf '%%s\n' "$MSG" > "$COMMIT_MSG_FILE"
    fi
fi
`, hookMarker, bin)
}

// Install writes the hook script. Returns an error if a third-party
// hook exists and overwrite is false.
func Install(overwrite bool) error {
	path, err := HookPath()
	if err != nil {
		return err
	}

	// Ensure hooks directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	thirdParty, err := ExistingHookIsThirdParty()
	if err != nil {
		return err
	}
	if thirdParty && !overwrite {
		return fmt.Errorf("an existing %s hook was found that was not installed by ai-commit; use --force to overwrite", hookName)
	}

	script := HookScript()
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		return fmt.Errorf("failed to write hook: %w", err)
	}
	return nil
}

// Uninstall removes the hook, but only if it was installed by ai-commit.
func Uninstall() error {
	path, err := HookPath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no %s hook is installed", hookName)
		}
		return err
	}

	if !strings.Contains(string(data), hookMarker) {
		return fmt.Errorf("the existing %s hook was not installed by ai-commit; refusing to remove", hookName)
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to remove hook: %w", err)
	}
	return nil
}
