package template

import (
	"strings"

	"github.com/renatogalera/ai-commit/pkg/git"
)

// ApplyTemplate replaces placeholders in the template with actual values:
// {COMMIT_MESSAGE} -> the AI-generated commit message
// {GIT_BRANCH} -> the current Git branch name
func ApplyTemplate(templateStr, commitMessage string) (string, error) {
	result := templateStr
	if strings.Contains(result, "{COMMIT_MESSAGE}") {
		result = strings.ReplaceAll(result, "{COMMIT_MESSAGE}", commitMessage)
	}

	if strings.Contains(result, "{GIT_BRANCH}") {
		branch, err := git.GetCurrentBranch(nil) // no special context needed here
		if err != nil {
			return "", err
		}
		result = strings.ReplaceAll(result, "{GIT_BRANCH}", branch)
	}

	return result, nil
}
