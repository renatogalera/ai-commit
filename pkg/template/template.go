package template

import (
	"context"
	"strings"

	"github.com/renatogalera/ai-commit/pkg/git"
)

// ApplyTemplate replaces well-known tokens in a commit template.
// Supported tokens:
//
//	{COMMIT_MESSAGE} - replaced with the generated commit message
//	{GIT_BRANCH}     - replaced with the current branch name
//	{TICKET_ID}      - replaced with a ticket ID extracted from the branch name
func ApplyTemplate(templateStr, commitMessage, ticketPattern string) (string, error) {
	result := templateStr
	if strings.Contains(result, "{COMMIT_MESSAGE}") {
		result = strings.ReplaceAll(result, "{COMMIT_MESSAGE}", commitMessage)
	}

	needsBranch := strings.Contains(result, "{GIT_BRANCH}") || strings.Contains(result, "{TICKET_ID}")
	var branch string
	if needsBranch {
		var err error
		branch, err = git.GetCurrentBranch(context.Background())
		if err != nil {
			return "", err
		}
	}

	if strings.Contains(result, "{GIT_BRANCH}") {
		result = strings.ReplaceAll(result, "{GIT_BRANCH}", branch)
	}
	if strings.Contains(result, "{TICKET_ID}") {
		ticketID := git.ExtractTicketID(branch, ticketPattern)
		result = strings.ReplaceAll(result, "{TICKET_ID}", ticketID)
	}
	return result, nil
}
