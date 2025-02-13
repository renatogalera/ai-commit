package versioner

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"golang.org/x/mod/semver"

	"github.com/renatogalera/ai-commit/pkg/ai"
)

// GetCurrentVersionTag retrieves the latest Git tag that follows semantic versioning using go-git.
func GetCurrentVersionTag(ctx context.Context) (string, error) {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}
	tagIter, err := repo.Tags()
	if err != nil {
		return "", fmt.Errorf("failed to get tags: %w", err)
	}
	var latestTag string
	err = tagIter.ForEach(func(ref *plumbing.Reference) error {
		tagName := ref.Name().Short()
		if strings.HasPrefix(tagName, "v") && semver.IsValid(tagName) {
			if latestTag == "" || semver.Compare(tagName, latestTag) > 0 {
				latestTag = tagName
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return latestTag, nil
}

// SuggestNextVersion uses OpenAI to suggest the next semantic version based on the commit message.
func SuggestNextVersion(ctx context.Context, currentVersion, commitMsg string, client ai.AIClient) (string, error) {
	if currentVersion == "" {
		currentVersion = "v0.0.0"
	}
	prompt := buildVersionPrompt(currentVersion, commitMsg)
	aiResponse, err := client.GetCommitMessage(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to get version suggestion: %w", err)
	}

	suggested, err := parseAiVersionSuggestion(aiResponse, currentVersion)
	if err != nil {
		return "", fmt.Errorf("failed to parse AI version suggestion: %w", err)
	}
	return suggested, nil
}

// CreateLocalTag creates a new Git tag locally using the specified new version tag.
func CreateLocalTag(ctx context.Context, newVersionTag string) error {
	if newVersionTag == "" {
		return errors.New("no version tag provided")
	}

	repo, err := git.PlainOpen(".")
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	headRef, err := repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD reference: %w", err)
	}

	_, err = repo.CreateTag(newVersionTag, headRef.Hash(), nil)
	if err != nil {
		return fmt.Errorf("failed to create tag %s: %w", newVersionTag, err)
	}

	return nil
}

// buildVersionPrompt constructs the prompt for the AI based on the current version and commit message.
func buildVersionPrompt(currentVersion, commitMsg string) string {
	return fmt.Sprintf(`
We are using semantic versioning, where a version is defined as MAJOR.MINOR.PATCH.
The current version is %s.
The latest commit message is:
"%s"

Based on the commit message, determine if the next version should be:
- a MAJOR update if it introduces breaking changes,
- a MINOR update if it adds new features,
- a PATCH update if it is a bug fix or minor improvement.

Please present the next version in the format vX.Y.Z without any extra explanation.
`, currentVersion, commitMsg)
}

// parseAiVersionSuggestion extracts a valid semantic version from the AI's response.
// If no valid version is found, it increments the patch version of the fallback version.
func parseAiVersionSuggestion(aiResponse, fallback string) (string, error) {
	re := regexp.MustCompile(`v?(\d+\.\d+\.\d+)`)
	match := re.FindStringSubmatch(aiResponse)
	if len(match) < 2 {
		return incrementPatch(fallback), nil
	}
	suggestedVersion := "v" + match[1]
	return suggestedVersion, nil
}

// incrementPatch increments the patch version of the given version tag.
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

// stripLeadingV removes a leading 'v' from the version string, if present.
func stripLeadingV(version string) string {
	if strings.HasPrefix(version, "v") {
		return strings.TrimPrefix(version, "v")
	}
	return version
}
