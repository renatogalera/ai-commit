package openai_compat

import (
    "context"
    "errors"
    "fmt"
    "strings"

    openai "github.com/openai/openai-go/v2"
    "github.com/openai/openai-go/v2/option"
    "github.com/renatogalera/ai-commit/pkg/ai"
)

// Client is a reusable OpenAI-compatible client (OpenAI, DeepSeek, etc.).
// It uses the official openai-go SDK and accepts a custom baseURL.
type Client struct {
    ai.BaseAIClient
    client openai.Client
    model  string
}

func NewCompatClient(provider, apiKey, model, baseURL string) *Client {
    // Build client with provided options.
    switch {
    case strings.TrimSpace(apiKey) != "" && strings.TrimSpace(baseURL) != "":
        c := openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(strings.TrimRight(baseURL, "/")))
        return &Client{BaseAIClient: ai.BaseAIClient{Provider: provider}, client: c, model: model}
    case strings.TrimSpace(apiKey) != "":
        c := openai.NewClient(option.WithAPIKey(apiKey))
        return &Client{BaseAIClient: ai.BaseAIClient{Provider: provider}, client: c, model: model}
    case strings.TrimSpace(baseURL) != "":
        c := openai.NewClient(option.WithBaseURL(strings.TrimRight(baseURL, "/")))
        return &Client{BaseAIClient: ai.BaseAIClient{Provider: provider}, client: c, model: model}
    default:
        c := openai.NewClient()
        return &Client{BaseAIClient: ai.BaseAIClient{Provider: provider}, client: c, model: model}
    }
}

func (c *Client) GetCommitMessage(ctx context.Context, prompt string) (string, error) {
    params := openai.ChatCompletionNewParams{
        Messages: []openai.ChatCompletionMessageParamUnion{
            openai.UserMessage(prompt),
        },
        Model: openai.ChatModel(c.model),
    }
    resp, err := c.client.Chat.Completions.New(ctx, params)
    if err != nil {
        return "", fmt.Errorf("failed to get chat completion: %w", err)
    }
    if len(resp.Choices) == 0 {
        return "", errors.New("no response from OpenAI-compatible provider")
    }
    return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

// StreamCommitMessage streams text deltas via onDelta and returns the final text.
func (c *Client) StreamCommitMessage(ctx context.Context, prompt string, onDelta func(string)) (string, error) {
    params := openai.ChatCompletionNewParams{
        Messages: []openai.ChatCompletionMessageParamUnion{
            openai.UserMessage(prompt),
        },
        Model: openai.ChatModel(c.model),
    }
    stream := c.client.Chat.Completions.NewStreaming(ctx, params)
    acc := openai.ChatCompletionAccumulator{}
    for stream.Next() {
        chunk := stream.Current()
        acc.AddChunk(chunk)
        if len(chunk.Choices) > 0 {
            if d := chunk.Choices[0].Delta.Content; d != "" {
                onDelta(d)
            }
        }
    }
    if err := stream.Err(); err != nil {
        // Return whatever was accumulated with error
        if len(acc.Choices) > 0 {
            return acc.Choices[0].Message.Content, err
        }
        return "", err
    }
    if len(acc.Choices) == 0 {
        return "", errors.New("no response from OpenAI-compatible provider")
    }
    return acc.Choices[0].Message.Content, nil
}

func (c *Client) SanitizeResponse(message, commitType string) string {
    return c.BaseAIClient.SanitizeResponse(message, commitType)
}

func (c *Client) MaybeSummarizeDiff(diff string, maxLength int) (string, bool) {
    return c.BaseAIClient.MaybeSummarizeDiff(diff, maxLength)
}

var _ ai.AIClient = (*Client)(nil)
var _ ai.StreamingAIClient = (*Client)(nil)
