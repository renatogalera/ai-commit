package prompt

import (
	"fmt"
	"strings"

	"github.com/renatogalera/ai-commit/pkg/committypes"
)

// DefaultPromptTemplate is used if no template is configured
const DefaultPromptTemplate = `Generate a git commit message following these rules:
- Use Conventional Commits format (e.g., 'feat: add new feature X', 'fix(login): handle edge case').
- Keep the subject line concise (ideally under 50 characters), in the imperative mood.
- If breaking changes exist, add 'BREAKING CHANGE:' in the body.
- After the subject line, add a blank line, then bullet points describing changes with '- '.
- Omit disclaimers, code blocks, or references to AI.
- Use the present tense and ensure clarity.
- Output only the commit message.
- Do NOT include the commit hash or branch name.
- When editing the README, ignore the diff and just let us know that it has been updated.
- Do not repeat topics.
- Do NOT begin your commit message with the word 'git' or references to it.
{COMMIT_TYPE_HINT}
- Write the message in {LANGUAGE}.
Here is the diff:

{DIFF}
{ADDITIONAL_CONTEXT}
`

// DefaultCodeReviewPromptTemplate template for code review prompts
const DefaultCodeReviewPromptTemplate = `Review the following code diff for potential issues, and provide suggestions, following these rules:
- Identify potential style issues, refactoring opportunities, and basic security risks if any.
- Focus on code quality and best practices.
- Provide concise suggestions in bullet points, prefixed with "- ".
- Be direct and avoid extraneous conversational text.
- Assume the perspective of a code reviewer offering constructive feedback to a developer.
- If no issues are found, explicitly state "No issues found."
- Language of the response MUST be {LANGUAGE}.

Diff:
{DIFF}
`

// BuildCommitPrompt ...
func BuildCommitPrompt(diff, language, commitType, additionalText, promptTemplate string) string {
	var sb strings.Builder

	finalTemplate := promptTemplate
	if finalTemplate == "" {
		finalTemplate = DefaultPromptTemplate
	}

	commitTypeHint := ""
	if commitType != "" && committypes.IsValidCommitType(commitType) {
		commitTypeHint = fmt.Sprintf("- Use the commit type '%s'.\n", commitType)
	}

	promptText := strings.ReplaceAll(finalTemplate, "{COMMIT_TYPE_HINT}", commitTypeHint)
	promptText = strings.ReplaceAll(promptText, "{LANGUAGE}", language)
	promptText = strings.ReplaceAll(promptText, "{DIFF}", diff)

	additionalContextStr := ""
	if additionalText != "" {
		additionalContextStr = "\n\n[Additional context provided by user]\n" + additionalText
	}
	promptText = strings.ReplaceAll(promptText, "{ADDITIONAL_CONTEXT}", additionalContextStr)

	sb.WriteString(promptText)
	return sb.String()
}

// BuildCodeReviewPrompt ...
func BuildCodeReviewPrompt(diff, language string, promptTemplate string) string {
	var sb strings.Builder

	finalTemplate := promptTemplate
	if finalTemplate == "" {
		finalTemplate = DefaultCodeReviewPromptTemplate
	}

	promptText := strings.ReplaceAll(finalTemplate, "{LANGUAGE}", language)
	promptText = strings.ReplaceAll(promptText, "{DIFF}", diff)

	sb.WriteString(promptText)
	return sb.String()
}
