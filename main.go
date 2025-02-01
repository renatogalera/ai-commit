package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// OpenAIChatRequest represents the request to the OpenAI Chat Completions API.
type OpenAIChatRequest struct {
	Model    string             `json:"model"`
	Messages []OpenAIChatMessage `json:"messages"`
}

// OpenAIChatMessage represents a message in the conversation.
type OpenAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIChatResponse represents the response from the OpenAI Chat Completions API.
type OpenAIChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// getGitDiff retrieves the staged diff from the git repository.
func getGitDiff() (string, error) {
	cmd := exec.Command("git", "diff", "--staged")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// checkGitRepository verifies that the current directory is inside a git repository.
func checkGitRepository() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

// filterLockFiles removes diff lines corresponding to common lock files.
func filterLockFiles(diff string) string {
	lines := strings.Split(diff, "\n")
	var filtered []string
	isLockFile := false
	regex := regexp.MustCompile(`^diff --git a/(.*/)?(yarn\.lock|pnpm-lock\.yaml|package-lock\.json)`)
	for _, line := range lines {
		if regex.MatchString(line) {
			isLockFile = true
			continue
		}
		if isLockFile && strings.HasPrefix(line, "diff --git") {
			isLockFile = false
		}
		if !isLockFile {
			filtered = append(filtered, line)
		}
	}
	return strings.Join(filtered, "\n")
}

// getCurrentBranch retrieves the current git branch.
func getCurrentBranch() (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// buildPrompt constructs the prompt for OpenAI based on the diff and provided options.
func buildPrompt(diff, language, commitType string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Write a professional git commit message based on the diff below in %s language", language))
	if commitType != "" {
		sb.WriteString(fmt.Sprintf(" with commit type '%s'.", commitType))
	} else {
		sb.WriteString(".")
	}
	sb.WriteString(" Do not preface the commit with anything, use the present tense and return a full sentence.")
	sb.WriteString("\n\n")
	sb.WriteString(diff)
	return sb.String()
}

// callOpenAI sends the prompt to the OpenAI API and returns the response message.
func callOpenAI(prompt, apiKey, model string) (string, error) {
	reqBody := OpenAIChatRequest{
		Model: model,
		Messages: []OpenAIChatMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	request, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var chatResp OpenAIChatResponse
	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		return "", err
	}
	if chatResp.Error != nil {
		return "", fmt.Errorf("OpenAI API error: %s", chatResp.Error.Message)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}
	return strings.TrimSpace(chatResp.Choices[0].Message.Content), nil
}

// addGitmoji prepends a gitmoji to the commit message based on commit type.
func addGitmoji(message, commitType string, addEmoji bool) string {
	if !addEmoji {
		return message
	}
	gitmojis := map[string]string{
		"feat":     "✨",
		"fix":      "🚑",
		"docs":     "📝",
		"style":    "💄",
		"refactor": "♻️",
		"test":     "✅",
		"chore":    "🔧",
	}
	if emoji, ok := gitmojis[strings.ToLower(commitType)]; ok {
		return fmt.Sprintf("%s %s", emoji, message)
	}
	return message
}

// applyTemplate replaces placeholders in the template with actual values.
func applyTemplate(template, commitMessage string) (string, error) {
	if !strings.Contains(template, "{COMMIT_MESSAGE}") {
		return commitMessage, nil
	}
	finalMsg := strings.ReplaceAll(template, "{COMMIT_MESSAGE}", commitMessage)
	if strings.Contains(finalMsg, "{GIT_BRANCH}") {
		branch, err := getCurrentBranch()
		if err != nil {
			return "", err
		}
		finalMsg = strings.ReplaceAll(finalMsg, "{GIT_BRANCH}", branch)
	}
	return strings.TrimSpace(finalMsg), nil
}

// commitChanges runs the git commit command with the provided commit message.
func commitChanges(commitMessage string) error {
	cmd := exec.Command("git", "commit", "-F", "-")
	cmd.Stdin = strings.NewReader(commitMessage)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func main() {
	// Command-line flags
	apiKeyFlag := flag.String("apiKey", "", "OpenAI API key (or set OPENAI_API_KEY environment variable)")
	languageFlag := flag.String("language", "english", "Language for the commit message")
	commitTypeFlag := flag.String("commit-type", "", "Commit type (e.g. feat, fix, docs)")
	templateFlag := flag.String("template", "", "Commit message template (e.g. \"Modified {GIT_BRANCH} | {COMMIT_MESSAGE}\")")
	addEmojiFlag := flag.Bool("emoji", false, "Add a gitmoji to the commit message")
	forceFlag := flag.Bool("force", false, "Automatically create the commit without prompting")
	flag.Parse()

	apiKey := *apiKeyFlag
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		fmt.Println("Error: OpenAI API key must be provided via --apiKey flag or OPENAI_API_KEY environment variable.")
		os.Exit(1)
	}

	if !checkGitRepository() {
		fmt.Println("Error: This is not a git repository.")
		os.Exit(1)
	}

	diff, err := getGitDiff()
	if err != nil {
		fmt.Printf("Error getting git diff: %v\n", err)
		os.Exit(1)
	}

	originalDiff := diff
	diff = filterLockFiles(diff)
	if diff == "" {
		fmt.Println("No changes to commit (after filtering lock files). Did you stage your changes?")
		os.Exit(0)
	}
	if diff != originalDiff {
		fmt.Println("Note: Changes in lock files will be committed but not analyzed for commit message generation.")
	}

	prompt := buildPrompt(diff, *languageFlag, *commitTypeFlag)
	fmt.Println("Prompt sent to OpenAI:")
	fmt.Println(prompt)

	commitMsg, err := callOpenAI(prompt, apiKey, "gpt-4o-mini")
	if err != nil {
		fmt.Printf("Error calling OpenAI: %v\n", err)
		os.Exit(1)
	}

	commitMsg = addGitmoji(commitMsg, *commitTypeFlag, *addEmojiFlag)

	if *templateFlag != "" {
		commitMsg, err = applyTemplate(*templateFlag, commitMsg)
		if err != nil {
			fmt.Printf("Error applying template: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("Proposed Commit Message:")
	fmt.Println("------------------------------")
	fmt.Println(commitMsg)
	fmt.Println("------------------------------")

	if !*forceFlag {
		fmt.Print("Do you want to continue? (y/n): ")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading input: %v\n", err)
			os.Exit(1)
		}
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			fmt.Println("Commit aborted.")
			os.Exit(0)
		}
	}

	err = commitChanges(commitMsg)
	if err != nil {
		fmt.Printf("Error creating commit: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Commit created successfully!")
}

