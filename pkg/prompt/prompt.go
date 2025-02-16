package prompt

import (
	"fmt"
	"strings"

	gogitobj "github.com/go-git/go-git/v5/plumbing/object"

	"github.com/renatogalera/ai-commit/pkg/committypes"
)

// DefaultPromptTemplate is used if no template is configured.
const DefaultPromptTemplate = `Generate a git commit message that is clear, concise, and follows the Conventional Commits format:
- Use the format "type: subject" (e.g., "fix: correct error handling").
- Keep the subject line under 50 characters and in the imperative mood.
- If there are breaking changes, include "BREAKING CHANGE:" in the body.
- After the subject line, leave a blank line and then list key changes with bullet points.
- Do not include extraneous details such as commit hash, branch name, spacing details, or formatting guidelines.
- Avoid repeating information.
{COMMIT_TYPE_HINT}
- Write the message in {LANGUAGE}.

Diff:
{DIFF}
{ADDITIONAL_CONTEXT}
`

// DefaultCodeReviewPromptTemplate template for code review prompts.
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

// DefaultCommitStyleReviewPromptTemplate template for commit message style review prompts.
const DefaultCommitStyleReviewPromptTemplate = `Review the following commit message for clarity, informativeness, and adherence to best practices. Provide feedback in bullet points if the message is lacking in any way. Focus on these aspects:

- **Clarity**: Is the message clear and easy to understand? Would someone unfamiliar with the changes easily grasp the intent?
- **Informativeness**: Does the message provide sufficient context about *what* and *why* the change is being made? Does it go beyond just *how* the code was changed?
- **Diff Reflection**: Does the commit message accurately and adequately reflect the changes present in the Git diff? Is it more than just a superficial description?
- **Semantic Feedback**: If the message is vague or superficial, provide specific, actionable feedback to improve it (e.g., "This message is too vague; specify *why* this change is necessary", "Explain the impact of this change on the user").

If the commit message is well-written and meets these criteria, respond with the phrase: "No issues found."

Commit Message to Review:
{COMMIT_MESSAGE}

Language for feedback MUST be {LANGUAGE}.
`

const defaultCommitSummaryTemplate = `Summarize the following git commit in markdown format.
Use "###" to denote section titles. Include:

### General Summary
- Main purpose or key changes

### Detailed Changes
- Any noteworthy details (e.g., new features, bug fixes, refactors)

### Impact and Considerations
- Overview of how it affects the codebase and any considerations.

Commit Information:
Author: {AUTHOR}
Date: {DATE}
Commit Message:
{COMMIT_MSG}

Diff:
{DIFF}
`

// buildCommitSummaryPrompt constructs the prompt used to ask the AI for a summary.
func BuildCommitSummaryPrompt(commit *gogitobj.Commit, diffStr, customPromptTemplate string) string {

	templateUsed := defaultCommitSummaryTemplate
	if strings.TrimSpace(customPromptTemplate) != "" {
		templateUsed = customPromptTemplate
	}

	promptText := strings.ReplaceAll(templateUsed, "{AUTHOR}", commit.Author.Name)
	promptText = strings.ReplaceAll(promptText, "{DATE}", commit.Author.When.Format("Mon Jan 2 15:04:05 MST 2006"))
	promptText = strings.ReplaceAll(promptText, "{COMMIT_MSG}", commit.Message)
	promptText = strings.ReplaceAll(promptText, "{DIFF}", diffStr)

	return promptText
}

// BuildCommitPrompt builds the prompt for commit message generation.
func BuildCommitPrompt(diff string, language string, commitType string, additionalText string, promptTemplate string) string {
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

// BuildCodeReviewPrompt builds the prompt for code review.
func BuildCodeReviewPrompt(diff string, language string, promptTemplate string) string {
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

// BuildCommitStyleReviewPrompt builds the prompt for commit message style review.
func BuildCommitStyleReviewPrompt(commitMsg string, language string, promptTemplate string) string {
	var sb strings.Builder

	finalTemplate := promptTemplate
	if finalTemplate == "" {
		finalTemplate = DefaultCommitStyleReviewPromptTemplate
	}

	promptText := strings.ReplaceAll(finalTemplate, "{LANGUAGE}", language)
	promptText = strings.ReplaceAll(promptText, "{COMMIT_MESSAGE}", commitMsg)

	sb.WriteString(promptText)
	return sb.String()
}
