package ai

import "context"

type AIClient interface {
	GetCommitMessage(ctx context.Context, prompt string) (string, error)
}
