package template

import (
	"context"
	"strings"

	"github.com/renatogalera/ai-commit/pkg/git"
)

// ApplyTemplate replaces well-known tokens in a commit template.
// Supported tokens:
//   {COMMIT_MESSAGE} - replaced with the generated commit message
//   {GIT_BRANCH}     - replaced with the current branch name
func ApplyTemplate(templateStr, commitMessage string) (string, error) {
	result := templateStr
	if strings.Contains(result, "{COMMIT_MESSAGE}") {
		result = strings.ReplaceAll(result, "{COMMIT_MESSAGE}", commitMessage)
	}
	if strings.Contains(result, "{GIT_BRANCH}") {
		branch, err := git.GetCurrentBranch(context.Background())
		if err != nil {
			return "", err
		}
		result = strings.ReplaceAll(result, "{GIT_BRANCH}", branch)
	}
	return result, nil
}
