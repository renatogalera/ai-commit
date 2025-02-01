package template

import (
	"strings"

	"github.com/renatogalera/ai-commit/pkg/git"
)

func ApplyTemplate(template, commitMessage string) (string, error) {
	if !strings.Contains(template, "{COMMIT_MESSAGE}") {
		return commitMessage, nil
	}
	finalMsg := strings.ReplaceAll(template, "{COMMIT_MESSAGE}", commitMessage)
	if strings.Contains(finalMsg, "{GIT_BRANCH}") {
		branch, err := git.GetCurrentBranch()
		if err != nil {
			return "", err
		}
		finalMsg = strings.ReplaceAll(finalMsg, "{GIT_BRANCH}", branch)
	}
	return strings.TrimSpace(finalMsg), nil
}
