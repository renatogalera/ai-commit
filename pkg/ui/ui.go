package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/renatogalera/ai-commit/pkg/git"
	"github.com/renatogalera/ai-commit/pkg/openai"
	"github.com/renatogalera/ai-commit/pkg/template"
)

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

type Model struct {
	state         uiState
	commitMsg     string
	result        string
	spinner       spinner.Model
	prompt        string
	apiKey        string
	commitType    string
	template      string
	selectedIndex int
	commitTypes   []string
}

func NewUIModel(commitMsg, prompt, apiKey, commitType, tmpl string) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return Model{
		state:         stateShowCommit,
		commitMsg:     commitMsg,
		prompt:        prompt,
		apiKey:        apiKey,
		commitType:    commitType,
		template:      tmpl,
		spinner:       s,
		selectedIndex: 0,
		commitTypes: []string{
			"feat", "fix", "docs", "refactor", "chore",
			"test", "style", "build", "perf", "ci",
		},
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func NewProgram(model Model) *tea.Program {
	return tea.NewProgram(model, tea.WithAltScreen())
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
				m.state = stateGenerating
				m.spinner = spinner.New()
				m.spinner.Spinner = spinner.Dot
				return m, regenCmd(m.prompt, m.apiKey, m.commitType, m.template)
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
				m.commitType = m.commitTypes[m.selectedIndex]
				m.state = stateGenerating
				m.spinner = spinner.New()
				m.spinner.Spinner = spinner.Dot
				return m, regenCmd(m.prompt, m.apiKey, m.commitType, m.template)
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

func (m Model) View() string {
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

func commitCmd(commitMsg string) tea.Cmd {
	return func() tea.Msg {
		err := git.CommitChanges(commitMsg)
		return commitResultMsg{err: err}
	}
}

func regenCmd(prompt, apiKey, commitType, tmpl string) tea.Cmd {
	return func() tea.Msg {
		msg, err := regenerate(prompt, apiKey, commitType, tmpl)
		return regenMsg{msg: msg, err: err}
	}
}

func regenerate(prompt, apiKey, commitType, tmpl string) (string, error) {
	result, err := openai.GetChatCompletion(prompt, apiKey)
	if err != nil {
		return "", err
	}
	result = openai.SanitizeOpenAIResponse(result, commitType)
	result = openai.AddGitmoji(result, commitType)
	if tmpl != "" {
		result, err = template.ApplyTemplate(tmpl, result)
		if err != nil {
			return "", err
		}
	}
	return result, nil
}
