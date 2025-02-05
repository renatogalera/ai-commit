package openai

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	gogpt "github.com/sashabaranov/go-openai"

	"github.com/renatogalera/ai-commit/pkg/ai"
	"github.com/renatogalera/ai-commit/pkg/committypes"
)

type OpenAIClient struct {
	client *gogpt.Client
}

func NewOpenAIClient(client *gogpt.Client) *OpenAIClient {
	return &OpenAIClient{client: client}
}

func (oc *OpenAIClient) GetCommitMessage(ctx context.Context, prompt string) (string, error) {
	req := gogpt.ChatCompletionRequest{
		Model: gogpt.GPT4oLatest,
		Messages: []gogpt.ChatCompletionMessage{
			{
				Role:    gogpt.ChatMessageRoleUser,
				Content: prompt,
			},
		},
	}
	resp, err := oc.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get chat completion: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("no response from OpenAI")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
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
		lines := strings.SplitN(msg, "\n", 2)
		if len(lines) > 0 {
			sanitizePattern := regexp.MustCompile(`^(?:(\p{So}|\p{Sk}|:\w+:)\s*)?(` + committypes.TypesRegexPattern() + `)(\([^)]+\))?:\s*`)
			lines[0] = sanitizePattern.ReplaceAllString(lines[0], "")
		}
		msg = strings.Join(lines, "\n")
	}
	return strings.TrimSpace(msg)
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
	emojiPattern := committypes.BuildRegexPatternWithEmoji()
	if matches := emojiPattern.FindStringSubmatch(message); len(matches) > 0 {
		cleanMsg := emojiPattern.ReplaceAllString(message, "")
		return fmt.Sprintf("%s: %s", prefix, strings.TrimSpace(cleanMsg))
	}
	return fmt.Sprintf("%s: %s", prefix, message)
}

var _ ai.AIClient = (*OpenAIClient)(nil)
