package anthropic

import (
	"context"
	"errors"
	"fmt"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/renatogalera/ai-commit/pkg/ai"
)

type AnthropicClient struct {
    ai.BaseAIClient
    client anthropic.Client
    model  string
}

func NewAnthropicClient(provider, apiKey, model, baseURL string) (*AnthropicClient, error) {
    if strings.TrimSpace(apiKey) == "" {
        return nil, errors.New("anthropic API key is required")
    }
    var opts []option.RequestOption
    opts = append(opts, option.WithAPIKey(apiKey))
    if strings.TrimSpace(baseURL) != "" {
        opts = append(opts, option.WithBaseURL(baseURL))
    }
    c := anthropic.NewClient(opts...)
    return &AnthropicClient{
        BaseAIClient: ai.BaseAIClient{Provider: provider},
        client:       c,
        model:        model,
    }, nil
}

func (ac *AnthropicClient) GetCommitMessage(ctx context.Context, prompt string) (string, error) {
    params := anthropic.MessageNewParams{
        MaxTokens: 1024,
        Messages: []anthropic.MessageParam{
            anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
        },
        Model: anthropic.Model(ac.model),
    }
    resp, err := ac.client.Messages.New(ctx, params)
    if err != nil {
        return "", fmt.Errorf("failed to get message from Anthropic: %w", err)
    }
    if resp == nil || len(resp.Content) == 0 {
        return "", errors.New("no response from Anthropic")
    }
    var sb strings.Builder
    for _, blk := range resp.Content {
        switch v := blk.AsAny().(type) {
        case anthropic.TextBlock:
            sb.WriteString(v.Text)
        }
    }
    msg := strings.TrimSpace(sb.String())
    if msg == "" {
        return "", errors.New("empty response from Anthropic")
    }
    return msg, nil
}

// StreamCommitMessage streams text deltas from Anthropic SDK.
func (ac *AnthropicClient) StreamCommitMessage(ctx context.Context, prompt string, onDelta func(string)) (string, error) {
    params := anthropic.MessageNewParams{
        MaxTokens: 1024,
        Messages: []anthropic.MessageParam{
            anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
        },
        Model: anthropic.Model(ac.model),
    }
    stream := ac.client.Messages.NewStreaming(ctx, params)
    msg := anthropic.Message{}
    for stream.Next() {
        event := stream.Current()
        if err := msg.Accumulate(event); err != nil {
            return "", err
        }
        // Try to emit text deltas when available
        switch ev := event.AsAny().(type) {
        case anthropic.ContentBlockDeltaEvent:
            switch d := ev.Delta.AsAny().(type) {
            case anthropic.TextDelta:
                if d.Text != "" {
                    onDelta(d.Text)
                }
            }
        }
    }
    if err := stream.Err(); err != nil {
        // return whatever we have with error
        var sb strings.Builder
        for _, blk := range msg.Content {
            switch v := blk.AsAny().(type) {
            case anthropic.TextBlock:
                sb.WriteString(v.Text)
            }
        }
        return sb.String(), err
    }
    // Build final text
    var sb strings.Builder
    for _, blk := range msg.Content {
        switch v := blk.AsAny().(type) {
        case anthropic.TextBlock:
            sb.WriteString(v.Text)
        }
    }
    return sb.String(), nil
}

func (ac *AnthropicClient) SanitizeResponse(message, commitType string) string {
    return ac.BaseAIClient.SanitizeResponse(message, commitType)
}

func (ac *AnthropicClient) MaybeSummarizeDiff(diff string, maxLength int) (string, bool) {
    return ac.BaseAIClient.MaybeSummarizeDiff(diff, maxLength)
}

var _ ai.AIClient = (*AnthropicClient)(nil)
var _ ai.StreamingAIClient = (*AnthropicClient)(nil)
