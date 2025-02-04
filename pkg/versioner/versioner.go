package versioner

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"golang.org/x/mod/semver"

	"github.com/renatogalera/ai-commit/pkg/openai"
	gogpt "github.com/sashabaranov/go-openai"
)

// GetCurrentVersionTag retrieves the most recent Git tag that matches semantic versioning using go-git.
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
func SuggestNextVersion(ctx context.Context, currentVersion, commitMsg string, client *gogpt.Client) (string, error) {
	if currentVersion == "" {
		currentVersion = "v0.0.0"
	}
	prompt := buildVersionPrompt(currentVersion, commitMsg)
	aiResponse, err := openai.GetChatCompletion(ctx, client, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to get version suggestion: %w", err)
	}

	suggested, err := parseAiVersionSuggestion(aiResponse, currentVersion)
	if err != nil {
		return "", fmt.Errorf("failed to parse AI version suggestion: %w", err)
	}
	return suggested, nil
}

// TagAndPush creates a new Git tag and pushes it to the remote repository using go-git.
func TagAndPush(ctx context.Context, newVersionTag string) error {
	if newVersionTag == "" {
		return errors.New("no version tag provided to TagAndPush")
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
	err = repo.PushContext(ctx, &git.PushOptions{
		RemoteName: "origin",
		RefSpecs: []config.RefSpec{
			config.RefSpec("refs/tags/" + newVersionTag + ":refs/tags/" + newVersionTag),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to push tag %s: %w", newVersionTag, err)
	}
	return nil
}

// RunGoReleaser executes GoReleaser to create and publish release artifacts.
func RunGoReleaser(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "goreleaser", "release")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("goreleaser failed: %v\n%s", err, string(output))
	}
	return nil
}

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

func parseAiVersionSuggestion(aiResponse, fallback string) (string, error) {
	re := regexp.MustCompile(`v?(\d+\.\d+\.\d+)`)
	match := re.FindStringSubmatch(aiResponse)
	if len(match) < 2 {
		return incrementPatch(fallback), nil
	}
	suggestedVersion := "v" + match[1]
	return suggestedVersion, nil
}

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

func stripLeadingV(version string) string {
	if strings.HasPrefix(version, "v") {
		return strings.TrimPrefix(version, "v")
	}
	return version
}
