package versioner

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/renatogalera/ai-commit/pkg/openai"
)

// GetCurrentVersionTag retrieves the most recent git tag that looks like vX.Y.Z
// If no tag exists, returns an empty string.
func GetCurrentVersionTag() (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
	out, err := cmd.Output()
	if err != nil {
		// If no tags exist, we can return an empty string (not necessarily an error)
		if strings.Contains(strings.ToLower(err.Error()), "no names found") {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// SuggestNextVersion uses OpenAI to suggest a semantic version bump based on the latest commit message.
func SuggestNextVersion(ctx context.Context, currentVersion, commitMsg, apiKey string) (string, error) {
	if currentVersion == "" {
		currentVersion = "v0.0.0"
	}
	prompt := buildVersionPrompt(currentVersion, commitMsg)
	aiResponse, err := openai.GetChatCompletion(ctx, prompt, apiKey)
	if err != nil {
		return "", err
	}

	suggested, err := parseAiVersionSuggestion(aiResponse, currentVersion)
	if err != nil {
		return "", err
	}
	return suggested, nil
}

// TagAndPush creates a new git tag and pushes it to origin.
func TagAndPush(newVersionTag string) error {
	if newVersionTag == "" {
		return errors.New("no version tag provided to TagAndPush")
	}

	cmd := exec.Command("git", "tag", newVersionTag)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create tag %s: %w", newVersionTag, err)
	}

	cmd = exec.Command("git", "push", "origin", newVersionTag)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to push tag %s: %w", newVersionTag, err)
	}
	return nil
}

// RunGoReleaser runs a local goreleaser release --rm-dist, building and publishing artifacts
func RunGoReleaser() error {
	cmd := exec.Command("goreleaser", "release", "--rm-dist")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("goreleaser failed: %v\n%s", err, string(out))
	}
	return nil
}

// buildVersionPrompt creates an OpenAI prompt to ask for semantic version suggestions
func buildVersionPrompt(currentVersion, commitMsg string) string {
	// You can adjust instructions to better match your style or requirements
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

// parseAiVersionSuggestion looks for a version in the AI's response and does a sanity check
func parseAiVersionSuggestion(aiResponse, fallback string) (string, error) {
	// A simple regex to capture something like v1.2.3
	re := regexp.MustCompile(`v?(\d+\.\d+\.\d+)`)
	match := re.FindStringSubmatch(aiResponse)
	if len(match) < 2 {
		// If we can't parse a version from AI, fallback to a patch bump
		return incrementPatch(fallback), nil
	}
	suggestedVersion := "v" + match[1]
	return suggestedVersion, nil
}

// incrementPatch is a fallback if we can't parse the AI’s suggestion
func incrementPatch(versionTag string) string {
	ver := stripLeadingV(versionTag)
	parts := strings.Split(ver, ".")
	if len(parts) != 3 {
		// fallback
		return "v0.0.1"
	}
	patch, _ := atoi(parts[2])
	parts[2] = itoa(patch + 1)
	return "v" + strings.Join(parts, ".")
}

func stripLeadingV(version string) string {
	if strings.HasPrefix(version, "v") {
		return strings.TrimPrefix(version, "v")
	}
	return version
}

// minimal atoi / itoa to avoid additional imports; you can use strconv
func atoi(s string) (int, error) {
	var n int
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, errors.New("non-digit character")
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var sb strings.Builder
	isNegative := i < 0
	if isNegative {
		i = -i
	}
	for i > 0 {
		sb.WriteByte(byte('0' + i%10))
		i /= 10
	}
	res := []rune(sb.String())
	// reverse the runes
	for i, j := 0, len(res)-1; i < j; i, j = i+1, j-1 {
		res[i], res[j] = res[j], res[i]
	}
	if isNegative {
		return "-" + string(res)
	}
	return string(res)
}

