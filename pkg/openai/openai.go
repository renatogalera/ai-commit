package openai

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"regexp"

	gogpt "github.com/sashabaranov/go-openai"

	"github.com/renatogalera/ai-commit/pkg/committypes"
)

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
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("no response from OpenAI")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

func BuildPrompt(diff, language, commitType string) string {
	var sb strings.Builder
	sb.WriteString("Generate a git commit message following the Conventional Commits specification. ")
	sb.WriteString("The commit message must include a short subject line starting with the commit type (e.g., 'feat: Add new feature'), followed by a blank line, and then a detailed body. ")
	sb.WriteString("For the body, list each change as a separate bullet point, starting with a hyphen ('-'). ")
	sb.WriteString("Write using the present tense and ensure clarity. Output only the commit message with no additional text. ")
	sb.WriteString("Ignore changes to lock files (e.g., go.mod, go.sum). ")
	if commitType != "" {
		sb.WriteString(fmt.Sprintf("Use the commit type '%s'. ", commitType))
	}
	sb.WriteString(fmt.Sprintf("Write the message in %s. ", language))
	sb.WriteString("Here is the diff:\n\n")
	sb.WriteString(diff)
	return sb.String()
}

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

func SanitizeOpenAIResponse(msg, commitType string) string {
	msg = strings.ReplaceAll(msg, "```", "")
	msg = strings.TrimSpace(msg)

	if commitType != "" {
		pattern := regexp.MustCompile(`^(?:(\p{Emoji_Presentation}|\p{So}|\p{Sk}|:\w+:)\s*)?(` + committypes.TypesRegexPattern() + `):\s*|(` + committypes.TypesRegexPattern() + `):\s*`)
		lines := strings.SplitN(msg, "\n", 2)
		if len(lines) > 0 {
			lines[0] = pattern.ReplaceAllString(lines[0], "")
		}
		msg = strings.Join(lines, "\n")
		msg = strings.TrimSpace(msg)
	}
	return msg
}

func AddGitmoji(message, commitType string) string {
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

	emojiPattern := committypes.BuildRegexPatternWithEmoji()
	matches := emojiPattern.FindStringSubmatch(message)
	if len(matches) > 0 && matches[1] != "" {
		return message
	}

	if len(matches) > 0 {
		newMessage := emojiPattern.ReplaceAllString(message, fmt.Sprintf("%s:", prefix))
		return newMessage
	}
	return fmt.Sprintf("%s: %s", prefix, message)
}
