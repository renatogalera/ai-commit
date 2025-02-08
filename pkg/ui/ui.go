package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/renatogalera/ai-commit/pkg/ai"
	"github.com/renatogalera/ai-commit/pkg/committypes"
	"github.com/renatogalera/ai-commit/pkg/git"
	"github.com/renatogalera/ai-commit/pkg/openai"
	"github.com/renatogalera/ai-commit/pkg/prompt"
	"github.com/renatogalera/ai-commit/pkg/template"
)

type uiState int

const (
	stateShowCommit uiState = iota
	stateGenerating
	stateCommitting
	stateResult
	stateSelectType
	stateEditing
	stateEditingPrompt
)

type commitResultMsg struct {
	err error
}

type regenMsg struct {
	msg string
	err error
}

type Model struct {
	state         uiState
	commitMsg     string
	result        string
	spinner       spinner.Model
	diff          string
	language      string
	prompt        string
	commitType    string
	template      string
	enableEmoji   bool
	aiClient      ai.AIClient
	selectedIndex int
	commitTypes   []string
	regenCount    int
	maxRegens     int
	textarea      textarea.Model
}

var helpStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("212")).
	Bold(true)

func NewUIModel(commitMsg string, diff string, language string, prompt string, commitType string, tmpl string, enableEmoji bool, client ai.AIClient) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot

	ta := textarea.New()
	ta.Placeholder = "Edite aqui..."
	ta.Prompt = "> "
	ta.CharLimit = 0
	ta.SetWidth(50)
	ta.SetHeight(10)
	ta.ShowLineNumbers = false

	return Model{
		state:         stateShowCommit,
		commitMsg:     commitMsg,
		diff:          diff,
		language:      language,
		prompt:        prompt,
		commitType:    commitType,
		template:      tmpl,
		enableEmoji:   enableEmoji,
		aiClient:      client,
		spinner:       s,
		selectedIndex: 0,
		commitTypes:   committypes.AllTypes(),
		regenCount:    0,
		maxRegens:     3,
		textarea:      ta,
	}
}

func NewProgram(model Model) *tea.Program {
	return tea.NewProgram(model, tea.WithAltScreen())
}

func (m Model) Init() tea.Cmd {
	return nil
}

type autoQuitMsg struct{}

func autoQuitCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return autoQuitMsg{}
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
				if m.regenCount >= m.maxRegens {
					m.result = fmt.Sprintf("Regenerations maximum (%d) reached. No more regenerations allowed.", m.maxRegens)
					m.state = stateResult
					return m, autoQuitCmd()
				}
				m.state = stateGenerating
				m.spinner = spinner.New()
				m.spinner.Spinner = spinner.Dot
				m.regenCount++
				return m, regenCmd(m.aiClient, m.prompt, m.commitType, m.template, m.enableEmoji)
			case "q", "ctrl+c":
				return m, tea.Quit
			case "t":
				m.state = stateSelectType
				return m, nil
			case "e":
				m.state = stateEditing
				m.textarea.SetValue(m.commitMsg)
				m.textarea.Focus()
				return m, nil
			case "p":
				m.state = stateEditingPrompt
				m.textarea.SetValue("")
				m.textarea.Focus()
				return m, nil
			}
		case stateSelectType:
			switch msg.String() {
			case "q", "esc", "ctrl+c":
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
				m.commitType = m.commitTypes[m.selectedIndex]
				m.state = stateGenerating
				m.spinner = spinner.New()
				m.spinner.Spinner = spinner.Dot
				m.regenCount++
				m.prompt = prompt.BuildPrompt(m.diff, m.language, m.commitType, "")
				return m, regenCmd(m.aiClient, m.prompt, m.commitType, m.template, m.enableEmoji)
			}
		case stateEditing:
			var tcmd tea.Cmd
			m.textarea, tcmd = m.textarea.Update(msg)
			cmd = tea.Batch(cmd, tcmd)
			switch msg.String() {
			case "esc":
				m.state = stateShowCommit
				return m, cmd
			case "ctrl+s":
				m.commitMsg = m.textarea.Value()
				m.state = stateShowCommit
				return m, cmd
			}
		case stateEditingPrompt:
			var tcmd tea.Cmd
			m.textarea, tcmd = m.textarea.Update(msg)
			cmd = tea.Batch(cmd, tcmd)
			switch msg.String() {
			case "esc":
				m.state = stateShowCommit
				return m, cmd
			case "ctrl+s":
				userPrompt := m.textarea.Value()
				m.state = stateGenerating
				m.spinner = spinner.New()
				m.spinner.Spinner = spinner.Dot
				m.regenCount++
				m.prompt = prompt.BuildPrompt(m.diff, m.language, m.commitType, userPrompt)
				return m, regenCmd(m.aiClient, m.prompt, m.commitType, m.template, m.enableEmoji)
			}
		case stateResult:
			return m, autoQuitCmd()
		}
	case regenMsg:
		if msg.err != nil {
			m.result = fmt.Sprintf("Error generating commit message: %v", msg.err)
			m.state = stateResult
			return m, autoQuitCmd()
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
		return m, autoQuitCmd()
	case autoQuitMsg:
		return m, tea.Quit
	case spinner.TickMsg:
		if m.state == stateGenerating || m.state == stateCommitting {
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}
	return m, cmd
}

func (m Model) View() string {
	switch m.state {
	case stateShowCommit:
		helpText := helpStyle.Render(
			"Press 'y' to commit, 'r' to regenerate,\n'e' to edit the message, 't' to change the commit type,\n'p' to add custom prompt text or 'q' to quit.",
		)
		return fmt.Sprintf("%s\n\n%s", m.commitMsg, helpText)
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
			b.WriteString(fmt.Sprintf("%s %s => %s\n", cursor, ct, ct))
		}
		b.WriteString("\nUse up/down (or j/k) to navigate, enter to select,\n'q' or esc to go back.\n")
		return b.String()
	case stateEditing:
		return fmt.Sprintf("Editing commit message (Press ESC to cancel, Ctrl+S to save):\n\n%s", m.textarea.View())
	case stateEditingPrompt:
		return fmt.Sprintf("Add custom prompt text (Press ESC to cancel, Ctrl+S to apply):\n\n%s", m.textarea.View())
	}
	return ""
}

func commitCmd(commitMsg string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := git.CommitChanges(ctx, commitMsg)
		return commitResultMsg{err: err}
	}
}

func regenCmd(client ai.AIClient, prompt string, commitType string, tmpl string, enableEmoji bool) tea.Cmd {
	return func() tea.Msg {
		msg, err := regenerate(prompt, client, commitType, tmpl, enableEmoji)
		return regenMsg{msg: msg, err: err}
	}
}

func regenerate(prompt string, client ai.AIClient, commitType string, tmpl string, enableEmoji bool) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	result, err := client.GetCommitMessage(ctx, prompt)
	if err != nil {
		return "", err
	}
	result = openai.SanitizeOpenAIResponse(result, commitType)
	if enableEmoji {
		result = ai.AddGitmoji(result, commitType)
	}
	if tmpl != "" {
		result, err = template.ApplyTemplate(tmpl, result)
		if err != nil {
			return "", err
		}
	}
	return result, nil
}
