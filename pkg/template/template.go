package template

import (
	"strings"

	"github.com/renatogalera/ai-commit/pkg/git"
)

func ApplyTemplate(templateStr, commitMessage string) (string, error) {
	result := templateStr
	if strings.Contains(result, "{COMMIT_MESSAGE}") {
		result = strings.ReplaceAll(result, "{COMMIT_MESSAGE}", commitMessage)
	}
	if strings.Contains(result, "{GIT_BRANCH}") {
		branch, err := git.GetCurrentBranch(nil)
		if err != nil {
			return "", err
		}
		result = strings.ReplaceAll(result, "{GIT_BRANCH}", branch)
	}
	return result, nil
}
