package testutil

import "context"

// MockAIClient is a configurable mock for ai.AIClient.
type MockAIClient struct {
	ProviderNameVal        string
	GetCommitMessageFunc   func(ctx context.Context, prompt string) (string, error)
	SanitizeResponseFunc   func(message, commitType string) string
	MaybeSummarizeDiffFunc func(diff string, maxLength int) (string, bool)
}

func (m *MockAIClient) ProviderName() string {
	return m.ProviderNameVal
}

func (m *MockAIClient) GetCommitMessage(ctx context.Context, prompt string) (string, error) {
	if m.GetCommitMessageFunc != nil {
		return m.GetCommitMessageFunc(ctx, prompt)
	}
	return "feat: mock commit message", nil
}

func (m *MockAIClient) SanitizeResponse(message, commitType string) string {
	if m.SanitizeResponseFunc != nil {
		return m.SanitizeResponseFunc(message, commitType)
	}
	return message
}

func (m *MockAIClient) MaybeSummarizeDiff(diff string, maxLength int) (string, bool) {
	if m.MaybeSummarizeDiffFunc != nil {
		return m.MaybeSummarizeDiffFunc(diff, maxLength)
	}
	if len(diff) > maxLength {
		return diff[:maxLength], true
	}
	return diff, false
}
