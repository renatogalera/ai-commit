package openai

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	gogpt "github.com/sashabaranov/go-openai"

	"github.com/renatogalera/ai-commit/pkg/committypes"
)

// sanitizePattern is a precompiled regex that removes the conventional commit
// prefix if it exists. We build it dynamically using valid commit types.
var sanitizePattern = regexp.MustCompile(`^(?:(\p{So}|\p{Sk}|:\w+:)\s*)?(` + committypes.TypesRegexPattern() + `)(\([^)]+\))?:\s*`)

// GetChatCompletion calls the OpenAI API using a shared *gogpt.Client
// and returns the generated string.
func GetChatCompletion(ctx context.Context, client *gogpt.Client, prompt string) (string, error) {
	req := gogpt.ChatCompletionRequest{
		Model: gogpt.GPT4oLatest,
		Messages: []gogpt.ChatCompletionMessage{
			{
				Role:    gogpt.ChatMessageRoleUser,
				Content: prompt,
			},
		},
	}

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get chat completion: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("no response from OpenAI")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

// BuildPrompt constructs the prompt for the OpenAI API based on the diff, language, commit type,
// and allows an additional custom text appended at the end if desired.
func BuildPrompt(diff, language, commitType, additionalText string) string {
	var sb strings.Builder
	sb.WriteString("Generate a git commit message following these rules:\n")
	sb.WriteString("- Use Conventional Commits (type(scope?): description).\n")
	sb.WriteString("- Keep the subject line concise (ideally under 50 characters), in the imperative mood.\n")
	sb.WriteString("- If breaking changes exist, add 'BREAKING CHANGE:' in the body.\n")
	sb.WriteString("- After the subject line, add a blank line, then bullet points describing changes with '- '.\n")
	sb.WriteString("- Omit disclaimers, code blocks, or references to AI.\n")
	sb.WriteString("- Use the present tense and ensure clarity.\n")
	sb.WriteString("- Output only the commit message.\n")
	if commitType != "" && committypes.IsValidCommitType(commitType) {
		sb.WriteString(fmt.Sprintf("- Use the commit type '%s'.\n", commitType))
	}
	sb.WriteString(fmt.Sprintf("- Write the message in %s.\n", language))
	sb.WriteString("Here is the diff:\n\n")
	sb.WriteString(diff)

	// If user wants to add custom prompt text, add it at the end.
	if additionalText != "" {
		sb.WriteString("\n\n[Additional context provided by user]\n")
		sb.WriteString(additionalText)
	}

	return sb.String()
}

// MaybeSummarizeDiff truncates the diff if it exceeds maxLength, appending a truncation notice.
func MaybeSummarizeDiff(diff string, maxLength int) (string, bool) {
	if len(diff) <= maxLength {
		return diff, false
	}
	truncated := diff[:maxLength]
	lastNewLine := strings.LastIndex(truncated, "\n")
	if lastNewLine != -1 {
		truncated = truncated[:lastNewLine]
	}
	truncated += "\n[... diff truncated for brevity ...]"
	return truncated, true
}

// SanitizeOpenAIResponse cleans the OpenAI response by removing code fences and
// removing any leading Conventional Commit tokens if the user already specified a type.
// It also strips any leading "git" token that sometimes appears in the first line.
func SanitizeOpenAIResponse(msg, commitType string) string {
	// Remove code fences
	msg = strings.ReplaceAll(msg, "```", "")
	msg = strings.TrimSpace(msg)

	lines := strings.Split(msg, "\n")
	if len(lines) > 0 {
		// If the first line starts with "git", remove it.
		// E.g. "git ", "git:", "git-"
		possibleGitPrefix := strings.ToLower(strings.TrimSpace(lines[0]))
		if strings.HasPrefix(possibleGitPrefix, "git") {
			// Remove only the "git" plus any trailing space/colon/dash
			// so we don't nuke the rest of the line.
			clean := strings.TrimSpace(
				strings.TrimPrefix(
					strings.TrimPrefix(lines[0], "git"),
					":",
				),
			)
			lines[0] = clean
		}

		// If commitType is non-empty, remove any raw type prefix from the first line.
		if commitType != "" {
			lines[0] = sanitizePattern.ReplaceAllString(lines[0], "")
		}
	}

	msg = strings.Join(lines, "\n")
	msg = strings.TrimSpace(msg)

	return msg
}

// AddGitmoji prepends an emoji to the commit message based on the commit type.
// If a conventional prefix is detected, it will be replaced with a new prefix.
func AddGitmoji(message, commitType string) string {
	// If commitType is empty, attempt to guess it from the message content.
	if commitType == "" {
		lowerMsg := strings.ToLower(message)
		switch {
		case strings.Contains(lowerMsg, "fix"):
			commitType = "fix"
		case strings.Contains(lowerMsg, "add"), strings.Contains(lowerMsg, "create"), strings.Contains(lowerMsg, "introduce"):
			commitType = "feat"
		case strings.Contains(lowerMsg, "doc"):
			commitType = "docs"
		case strings.Contains(lowerMsg, "refactor"):
			commitType = "refactor"
		case strings.Contains(lowerMsg, "test"):
			commitType = "test"
		case strings.Contains(lowerMsg, "perf"):
			commitType = "perf"
		case strings.Contains(lowerMsg, "build"):
			commitType = "build"
		case strings.Contains(lowerMsg, "ci"):
			commitType = "ci"
		case strings.Contains(lowerMsg, "chore"):
			commitType = "chore"
		}
	}

	// If we still don't have a commit type, return the message as is.
	if commitType == "" {
		return message
	}

	// Map each commit type to an emoji.
	gitmojis := map[string]string{
		"feat":     "✨",
		"fix":      "🐛",
		"docs":     "📚",
		"style":    "💎",
		"refactor": "♻️",
		"test":     "🧪",
		"chore":    "🔧",
		"perf":     "🚀",
		"build":    "📦",
		"ci":       "👷",
	}
	prefix := commitType
	if emoji, ok := gitmojis[commitType]; ok {
		prefix = fmt.Sprintf("%s %s", emoji, commitType)
	}

	// Build a regex that detects an existing conventional commit prefix.
	emojiPattern := committypes.BuildRegexPatternWithEmoji()
	if matches := emojiPattern.FindStringSubmatch(message); len(matches) > 0 {
		// Remove the existing prefix before adding the new one.
		cleanMsg := emojiPattern.ReplaceAllString(message, "")
		return fmt.Sprintf("%s: %s", prefix, strings.TrimSpace(cleanMsg))
	}

	return fmt.Sprintf("%s: %s", prefix, message)
}
