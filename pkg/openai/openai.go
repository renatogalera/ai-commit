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

// GetChatCompletion calls the OpenAI API to generate a chat completion based on the provided prompt.
func GetChatCompletion(ctx context.Context, prompt, apiKey string) (string, error) {
	client := gogpt.NewClient(apiKey)

	req := gogpt.ChatCompletionRequest{
		Model: gogpt.GPT4,
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

// BuildPrompt constructs the prompt for the OpenAI API based on the diff, language, and commit type.
func BuildPrompt(diff, language, commitType string) string {
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

// SanitizeOpenAIResponse cleans the OpenAI response by removing code blocks and unnecessary prefixes.
func SanitizeOpenAIResponse(msg, commitType string) string {
	msg = strings.ReplaceAll(msg, "```", "")
	msg = strings.TrimSpace(msg)

	if commitType != "" {
		// We remove the raw "feat:" etc. only if it matches our pattern.
		pattern := regexp.MustCompile(`^(?:(\p{So}|\p{Sk}|:\w+:)\s*)?(` + committypes.TypesRegexPattern() + `)(\([^)]+\))?:\s*`)
		lines := strings.SplitN(msg, "\n", 2)
		if len(lines) > 0 {
			lines[0] = pattern.ReplaceAllString(lines[0], "")
		}
		msg = strings.Join(lines, "\n")
		msg = strings.TrimSpace(msg)
	}

	return msg
}

// AddGitmoji prepends an emoji (if applicable) to the commit message based on the commit type.
// This updated version also captures an optional (scope) so that "feat(README): ..." will match properly.
func AddGitmoji(message, commitType string) string {
	// If the user didn't pass --commit-type, we do a naive guess.
	// This block is optional but often helpful.
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

	// If we can't detect a commit type, just return as-is.
	if commitType == "" {
		return message
	}

	// Map each commit type to a fitting emoji.
	prefix := commitType
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

	if emoji, ok := gitmojis[commitType]; ok {
		prefix = fmt.Sprintf("%s %s", emoji, commitType)
	}

	// If the commit message already starts with an emoji or recognized pattern, we skip re-injecting it.
	emojiPattern := committypes.BuildRegexPatternWithEmoji()
	matches := emojiPattern.FindStringSubmatch(message)
	if len(matches) > 0 {
		// matches[1] is the optional existing emoji
		// matches[3] is the commit type itself
		// matches[4] is the optional (scope)
		if matches[1] != "" {
			// There's already an emoji, so don't double up
			return message
		}
		// Preserve the scope if present
		scope := ""
		if len(matches) >= 5 {
			scope = matches[4]
		}
		// Replace everything up to the colon with "emoji + type + (scope):"
		newMessage := emojiPattern.ReplaceAllString(message, fmt.Sprintf("%s%s:", prefix, scope))
		return newMessage
	}

	// If the user typed something that doesn't match the pattern at all, just prefix it:
	return fmt.Sprintf("%s: %s", prefix, message)
}
