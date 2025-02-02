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

// We'll remove the old "git" strip logic and rely on the prompt instructions.
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

// BuildPrompt includes an extra rule about not starting with "git".
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

	// NEW RULE: no "git" at the start
	sb.WriteString("- Do NOT begin your commit message with the word 'git' or references to it.\n")

	if commitType != "" && committypes.IsValidCommitType(commitType) {
		sb.WriteString(fmt.Sprintf("- Use the commit type '%s'.\n", commitType))
	}
	sb.WriteString(fmt.Sprintf("- Write the message in %s.\n", language))

	sb.WriteString("Here is the diff:\n\n")
	sb.WriteString(diff)

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
// any leading conventional-commit token if we already have a commit type.
func SanitizeOpenAIResponse(msg, commitType string) string {
	// Remove code fences
	msg = strings.ReplaceAll(msg, "```", "")
	msg = strings.TrimSpace(msg)

	// If a commitType was specified, remove any raw type prefix in the first line.
	if commitType != "" {
		lines := strings.SplitN(msg, "\n", 2)
		if len(lines) > 0 {
			lines[0] = sanitizePattern.ReplaceAllString(lines[0], "")
		}
		msg = strings.Join(lines, "\n")
	}

	return strings.TrimSpace(msg)
}

// AddGitmoji prepends an emoji to the commit message based on the commit type.
func AddGitmoji(message, commitType string) string {
	// If commitType is empty, attempt to guess from the message
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

	if commitType == "" {
		return message
	}

	// Map each commit type to an emoji
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

	// Build a regex to detect an existing conventional commit prefix
	emojiPattern := committypes.BuildRegexPatternWithEmoji()
	if matches := emojiPattern.FindStringSubmatch(message); len(matches) > 0 {
		cleanMsg := emojiPattern.ReplaceAllString(message, "")
		return fmt.Sprintf("%s: %s", prefix, strings.TrimSpace(cleanMsg))
	}

	return fmt.Sprintf("%s: %s", prefix, message)
}
