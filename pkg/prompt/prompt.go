package prompt

import (
	"fmt"
	"strings"

	"github.com/renatogalera/ai-commit/pkg/committypes"
)

func BuildPrompt(diff, language, commitType, additionalText string) string {
	var sb strings.Builder
	sb.WriteString("Generate a git commit message following these rules:\n")
	sb.WriteString("- Use Conventional Commits (type(scope?): description).\n")
	sb.WriteString("- Keep the subject line concise (ideally under 50 characters), in the imperative mood.\n")
	sb.WriteString("- If breaking changes exist, add 'BREAKING CHANGE:' in the body.\n")
	sb.WriteString("- After the subject line, add a blank line, then bullet points describing changes with '- '.\n")
	sb.WriteString("- Omit disclaimers, code blocks, or references to AI.\n")
	sb.WriteString("- Use the present tense and ensure clarity.\n")
	sb.WriteString("- Output only the commit message.\n")
	sb.WriteString("- Do NOT begin your commit message with the word 'git' or references to it.\n")
	if commitType != "" && committypes.IsValidCommitType(commitType) {
		sb.WriteString(fmt.Sprintf("- Use the commit type '%s'.\n", commitType))
	}
	sb.WriteString(fmt.Sprintf("- Write the message in %s.\n", language))
	sb.WriteString("Here is the diff:\n\n")
	sb.WriteString(diff)
	if additionalText != "" {
		sb.WriteString("\n\n[Additional context provided by user]\n")
		sb.WriteString(additionalText)
	}
	return sb.String()
}

