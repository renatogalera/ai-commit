package versioner

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/renatogalera/ai-commit/pkg/openai"
)

// GetCurrentVersionTag retrieves the most recent git tag that matches semantic versioning.
func GetCurrentVersionTag(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "describe", "--tags", "--abbrev=0")
	out, err := cmd.Output()
	if err != nil {
		// If no tags are found, return empty string.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
			return "", nil
		}
		lowerErr := strings.ToLower(err.Error())
		if strings.Contains(lowerErr, "no names found") || strings.Contains(lowerErr, "no tags can describe") {
			return "", nil
		}
		return "", fmt.Errorf("error retrieving git tag: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// SuggestNextVersion uses OpenAI to suggest the next semantic version based on the commit message.
func SuggestNextVersion(ctx context.Context, currentVersion, commitMsg, apiKey string) (string, error) {
	if currentVersion == "" {
		currentVersion = "v0.0.0"
	}
	prompt := buildVersionPrompt(currentVersion, commitMsg)
	aiResponse, err := openai.GetChatCompletion(ctx, prompt, apiKey)
	if err != nil {
		return "", fmt.Errorf("failed to get version suggestion: %w", err)
	}

	suggested, err := parseAiVersionSuggestion(aiResponse, currentVersion)
	if err != nil {
		return "", fmt.Errorf("failed to parse AI version suggestion: %w", err)
	}
	return suggested, nil
}

// TagAndPush creates a new git tag and pushes it to the remote repository.
func TagAndPush(ctx context.Context, newVersionTag string) error {
	if newVersionTag == "" {
		return errors.New("no version tag provided to TagAndPush")
	}

	// Create tag.
	cmd := exec.CommandContext(ctx, "git", "tag", newVersionTag)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create tag %s: %w", newVersionTag, err)
	}

	// Push tag.
	cmd = exec.CommandContext(ctx, "git", "push", "origin", newVersionTag)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to push tag %s: %w", newVersionTag, err)
	}
	return nil
}

// RunGoReleaser executes GoReleaser to create and publish release artifacts.
func RunGoReleaser(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "goreleaser", "release")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("goreleaser failed: %v\n%s", err, string(out))
	}
	return nil
}

// buildVersionPrompt creates a prompt for OpenAI to suggest the next semantic version.
func buildVersionPrompt(currentVersion, commitMsg string) string {
	return fmt.Sprintf(`
We are using semantic versioning, where a version is defined as MAJOR.MINOR.PATCH.
The current version is %s.
The latest commit message is:
"%s"

Based on the commit message, determine if the next version is:
- MAJOR update if it introduces breaking changes
- MINOR update if it adds new features
- PATCH update if it's a fix or small improvement

Please output the next version in the format vX.Y.Z without extra explanation.
`, currentVersion, commitMsg)
}

// parseAiVersionSuggestion extracts the version from the AI's response and validates it.
func parseAiVersionSuggestion(aiResponse, fallback string) (string, error) {
	re := regexp.MustCompile(`v?(\d+\.\d+\.\d+)`)
	match := re.FindStringSubmatch(aiResponse)
	if len(match) < 2 {
		return incrementPatch(fallback), nil
	}
	suggestedVersion := "v" + match[1]
	return suggestedVersion, nil
}

// incrementPatch increments the patch version of the given version string.
func incrementPatch(versionTag string) string {
	ver := stripLeadingV(versionTag)
	parts := strings.Split(ver, ".")
	if len(parts) != 3 {
		return "v0.0.1"
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "v0.0.1"
	}
	parts[2] = strconv.Itoa(patch + 1)
	return "v" + strings.Join(parts, ".")
}

// stripLeadingV removes the leading 'v' from a version string if present.
func stripLeadingV(version string) string {
	if strings.HasPrefix(version, "v") {
		return strings.TrimPrefix(version, "v")
	}
	return version
}
