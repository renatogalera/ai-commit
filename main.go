package main

import (
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

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type OpenAIChatRequest struct {
	Model    string              `json:"model"`
	Messages []OpenAIChatMessage `json:"messages"`
}

type OpenAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

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

func getGitDiff() (string, error) {
	cmd := exec.Command("git", "diff", "--staged")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func checkGitRepository() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

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

func getCurrentBranch() (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func buildPrompt(diff, language, commitType string) string {
	var sb strings.Builder
	sb.WriteString("Generate a git commit message that follows the Conventional Commits specification. ")
	sb.WriteString("Use a short subject line preceded by the commit type (e.g., 'feat: Add new feature'), followed by a blank line, then a body explaining the changes. ")
	sb.WriteString("Focus on clarity, using the present tense. Only output the commit message with no additional text. ")
	if commitType != "" {
		sb.WriteString(fmt.Sprintf("Use the commit type '%s'. ", commitType))
	}
	sb.WriteString("Here is the diff:\n\n")
	sb.WriteString(diff)
	return sb.String()
}

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

// Removes triple backticks and, if a commitType is specified, removes any
// existing Conventional Commit prefix so we don't duplicate the type.
func sanitizeOpenAIResponse(msg, commitType string) string {
	msg = strings.ReplaceAll(msg, "```", "")
	msg = strings.TrimSpace(msg)
	if commitType != "" {
		// Regex that attempts to remove any leading "<emoji>? <type>: " or "<type>: "
		// from the first line only, if it exists.
		pattern := regexp.MustCompile(`^(?:(\p{Emoji_Presentation}|\p{So}|\p{Sk}|:\w+:)\s*)?(feat|fix|docs|chore|refactor|test|style|build|perf|ci):\s*|(feat|fix|docs|chore|refactor|test|style|build|perf|ci):\s*`)
		lines := strings.SplitN(msg, "\n", 2)
		if len(lines) > 0 {
			lines[0] = pattern.ReplaceAllString(lines[0], "")
		}
		msg = strings.Join(lines, "\n")
		msg = strings.TrimSpace(msg)
	}
	return msg
}

func addGitmoji(message, commitType string) string {
	// Determine commit type from message if not provided
	if commitType == "" {
		lowerMsg := strings.ToLower(message)
		switch {
		case strings.Contains(lowerMsg, "fix"):
			commitType = "fix"
		case strings.Contains(lowerMsg, "add"), strings.Contains(lowerMsg, "create"), strings.Contains(lowerMsg, "introduce"):
			commitType = "feat"
		case strings.Contains(lowerMsg, "doc"):
			commitType = "docs"
		case strings.Contains(lowerMsg, "refactor"):
			commitType = "refactor"
		case strings.Contains(lowerMsg, "test"):
			commitType = "test"
		case strings.Contains(lowerMsg, "perf"):
			commitType = "perf"
		case strings.Contains(lowerMsg, "build"):
			commitType = "build"
		case strings.Contains(lowerMsg, "ci"):
			commitType = "ci"
		case strings.Contains(lowerMsg, "chore"):
			commitType = "chore"
		}
	}
	if commitType == "" {
		return message
	}

	// Removed \p{Emoji_Presentation} since it's not supported in Go's regexp
	emojiTypePattern := regexp.MustCompile(`^((\p{So}|\p{Sk}|:\w+:)\s+)?(feat|fix|docs|chore|refactor|test|style|build|perf|ci):`)
	matches := emojiTypePattern.FindStringSubmatch(message)
	if len(matches) > 0 && matches[1] != "" {
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
		"perf":     "⚡",
		"build":    "👷",
		"ci":       "🔧",
	}
	lowerType := strings.ToLower(commitType)
	prefix := commitType
	if emoji, ok := gitmojis[lowerType]; ok {
		prefix = fmt.Sprintf("%s %s", emoji, commitType)
	}
	if len(matches) > 0 {
		newMessage := emojiTypePattern.ReplaceAllString(message, fmt.Sprintf("%s:", prefix))
		return newMessage
	}
	return fmt.Sprintf("%s: %s", prefix, message)
}

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

func commitChanges(commitMessage string) error {
	cmd := exec.Command("git", "commit", "-F", "-")
	cmd.Stdin = strings.NewReader(commitMessage)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type Config struct {
	Prompt     string
	APIKey     string
	CommitType string
	Template   string
}

func formatCommitMessage(msg string) string {
	// Split the commit message into header and body based on two consecutive newlines.
	parts := strings.SplitN(msg, "\n\n", 2)
	if len(parts) < 2 {
		return msg
	}
	header := parts[0]
	body := parts[1]

	// Split the body into lines.
	lines := strings.Split(body, "\n")
	totalLines := 0
	bulletLines := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			totalLines++
			if strings.HasPrefix(trimmed, "-") {
				bulletLines++
			}
		}
	}

	// If most non-empty lines already start with a hyphen, assume it's formatted.
	if totalLines > 0 && bulletLines >= totalLines/2 {
		return msg
	}

	// Otherwise, split the body into sentences based on periods.
	sentences := strings.Split(body, ".")
	var formattedSentences []string
	for _, sentence := range sentences {
		trimmed := strings.TrimSpace(sentence)
		if trimmed != "" {
			formattedSentences = append(formattedSentences, "- "+trimmed+".")
		}
	}
	formattedBody := strings.Join(formattedSentences, "\n")
	return header + "\n\n" + formattedBody
}

func generateCommitMessage(cfg Config) (string, error) {
	msg, err := callOpenAI(cfg.Prompt, cfg.APIKey, "chatgpt-4o-latest")
	if err != nil {
		return "", err
	}
	msg = sanitizeOpenAIResponse(msg, cfg.CommitType)
	msg = addGitmoji(msg, cfg.CommitType)
	if cfg.Template != "" {
		msg, err = applyTemplate(cfg.Template, msg)
		if err != nil {
			return "", err
		}
	}
	// Post-process the commit message to enforce bullet point formatting in the body.
	msg = formatCommitMessage(msg)
	return msg, nil
}

type uiState int

const (
	stateShowCommit uiState = iota
	stateGenerating
	stateCommitting
	stateResult
	stateSelectType
)

type commitResultMsg struct {
	err error
}

type regenMsg struct {
	msg string
	err error
}

type uiModel struct {
	state         uiState
	commitMsg     string
	result        string
	spinner       spinner.Model
	config        Config
	selectedIndex int
	commitTypes   []string
}

func newUIModel(commitMsg string, cfg Config) uiModel {
	s := spinner.New()
	s.Spinner = spinner.Dot

	return uiModel{
		state:         stateShowCommit,
		commitMsg:     commitMsg,
		config:        cfg,
		spinner:       s,
		selectedIndex: 0,
		commitTypes: []string{
			"feat", "fix", "docs", "refactor", "chore",
			"test", "style", "build", "perf", "ci",
		},
	}
}

func commitCmd(commitMsg string) tea.Cmd {
	return func() tea.Msg {
		err := commitChanges(commitMsg)
		return commitResultMsg{err: err}
	}
}

func regenCmd(cfg Config) tea.Cmd {
	return func() tea.Msg {
		msg, err := generateCommitMessage(cfg)
		return regenMsg{msg: msg, err: err}
	}
}

func (m uiModel) Init() tea.Cmd {
	return nil
}

func (m uiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.state {
		case stateShowCommit:
			switch msg.String() {
			case "y", "enter":
				m.state = stateCommitting
				return m, commitCmd(m.commitMsg)
			case "r":
				m.state = stateGenerating
				m.spinner = spinner.New()
				m.spinner.Spinner = spinner.Dot
				return m, regenCmd(m.config)
			case "q", "ctrl+c":
				return m, tea.Quit
			case "t":
				m.state = stateSelectType
				return m, nil
			}

		case stateSelectType:
			switch msg.String() {
			case "q", "ctrl+c":
				m.state = stateShowCommit
				return m, nil
			case "up", "k":
				if m.selectedIndex > 0 {
					m.selectedIndex--
				}
			case "down", "j":
				if m.selectedIndex < len(m.commitTypes)-1 {
					m.selectedIndex++
				}
			case "enter":
				m.config.CommitType = m.commitTypes[m.selectedIndex]
				m.state = stateGenerating
				m.spinner = spinner.New()
				m.spinner.Spinner = spinner.Dot
				return m, regenCmd(m.config)
			}

		case stateResult:
			return m, tea.Quit
		}

	case regenMsg:
		if msg.err != nil {
			m.result = fmt.Sprintf("Error generating commit message: %v", msg.err)
			m.state = stateResult
		} else {
			m.commitMsg = msg.msg
			m.state = stateShowCommit
		}

	case commitResultMsg:
		if msg.err != nil {
			m.result = fmt.Sprintf("Commit failed: %v", msg.err)
		} else {
			m.result = "Commit created successfully!"
		}
		m.state = stateResult

	case spinner.TickMsg:
		if m.state == stateGenerating || m.state == stateCommitting {
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m uiModel) View() string {
	switch m.state {
	case stateShowCommit:
		return fmt.Sprintf(
			"%s\n\nPress 'y' to commit, 'r' to regenerate,\n't' to change commit type, or 'q' to quit",
			m.commitMsg,
		)
	case stateGenerating:
		return fmt.Sprintf("Generating commit message... %s", m.spinner.View())
	case stateCommitting:
		return fmt.Sprintf("Committing... %s", m.spinner.View())
	case stateResult:
		return m.result
	case stateSelectType:
		var b strings.Builder
		b.WriteString("Select commit type:\n\n")
		for i, ct := range m.commitTypes {
			cursor := " "
			if i == m.selectedIndex {
				cursor = ">"
			}
			b.WriteString(fmt.Sprintf("%s %s\n", cursor, ct))
		}
		b.WriteString("\nUse up/down arrows (or j/k) to navigate, enter to select,\n'q' to go back.\n")
		return b.String()
	}
	return ""
}

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	apiKeyFlag := flag.String("apiKey", "", "OpenAI API key (or set OPENAI_API_KEY environment variable)")
	languageFlag := flag.String("language", "english", "Language for the commit message")
	commitTypeFlag := flag.String("commit-type", "", "Commit type (e.g. feat, fix, docs)")
	templateFlag := flag.String("template", "", "Commit message template (e.g. \"Modified {GIT_BRANCH} | {COMMIT_MESSAGE}\")")
	forceFlag := flag.Bool("force", false, "Automatically create the commit without prompting")
	flag.Parse()

	apiKey := *apiKeyFlag
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		log.Error().Msg("OpenAI API key must be provided via --apiKey flag or OPENAI_API_KEY environment variable")
		os.Exit(1)
	}

	if !checkGitRepository() {
		log.Error().Msg("This is not a git repository")
		os.Exit(1)
	}

	diff, err := getGitDiff()
	if err != nil {
		log.Error().Err(err).Msg("Error getting git diff")
		os.Exit(1)
	}

	originalDiff := diff
	diff = filterLockFiles(diff)
	if strings.TrimSpace(diff) == "" {
		fmt.Println("No changes to commit (after filtering lock files). Did you stage your changes?")
		os.Exit(0)
	}
	if diff != originalDiff {
		fmt.Println("Lock file changes will be committed but not analyzed for commit message generation.")
	}

	prompt := buildPrompt(diff, *languageFlag, *commitTypeFlag)

	cfg := Config{
		Prompt:     prompt,
		APIKey:     apiKey,
		CommitType: *commitTypeFlag,
		Template:   *templateFlag,
	}

	commitMsg, err := generateCommitMessage(cfg)
	if err != nil {
		log.Error().Err(err).Msg("Error generating commit message")
		os.Exit(1)
	}

	if *forceFlag {
		if err := commitChanges(commitMsg); err != nil {
			log.Error().Err(err).Msg("Error creating commit")
			os.Exit(1)
		}
		fmt.Println("Commit created successfully!")
		os.Exit(0)
	}

	model := newUIModel(commitMsg, cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if err := p.Start(); err != nil {
		log.Error().Err(err).Msg("Error running TUI program")
		os.Exit(1)
	}
}
