package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/renatogalera/ai-commit/pkg/committypes"
	"github.com/renatogalera/ai-commit/pkg/git"
	"github.com/renatogalera/ai-commit/pkg/openai"
	"github.com/renatogalera/ai-commit/pkg/template"
)

// uiState represents the different states of the UI.
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

// Model defines the state for the UI.
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
	enableEmoji   bool
}

// NewUIModel creates a new UI model with the provided parameters.
func NewUIModel(commitMsg, prompt, apiKey, commitType, tmpl string, enableEmoji bool) Model {
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
		commitTypes:   committypes.AllTypes(),
		enableEmoji:   enableEmoji,
	}
}

// Init is the initialization function for the Bubble Tea program.
func (m Model) Init() tea.Cmd {
	return nil
}

// NewProgram creates a new Bubble Tea program with the provided model.
func NewProgram(model Model) *tea.Program {
	return tea.NewProgram(model, tea.WithAltScreen())
}

// Update updates the UI model based on messages from Bubble Tea.
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
				return m, regenCmd(m.prompt, m.apiKey, m.commitType, m.template, m.enableEmoji)
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
				return m, regenCmd(m.prompt, m.apiKey, m.commitType, m.template, m.enableEmoji)
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
	newList, cmd := m.updateList(msg)
	m = newList.(Model)
	return m, cmd
}

// updateList is a helper function to update the list component if needed.
func (m Model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

// View renders the UI based on the current state.
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

// commitCmd returns a Bubble Tea command to commit changes with the provided commit message.
func commitCmd(commitMsg string) tea.Cmd {
	return func() tea.Msg {
		err := git.CommitChanges(commitMsg)
		return commitResultMsg{err: err}
	}
}

// regenCmd returns a command to regenerate the commit message using OpenAI.
func regenCmd(prompt, apiKey, commitType, tmpl string, enableEmoji bool) tea.Cmd {
	return func() tea.Msg {
		msg, err := regenerate(prompt, apiKey, commitType, tmpl, enableEmoji)
		return regenMsg{msg: msg, err: err}
	}
}

// regenerate calls the OpenAI API to generate a new commit message.
func regenerate(prompt, apiKey, commitType, tmpl string, enableEmoji bool) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	result, err := openai.GetChatCompletion(ctx, prompt, apiKey)
	if err != nil {
		return "", err
	}
	result = openai.SanitizeOpenAIResponse(result, commitType)
	if enableEmoji {
		result = openai.AddGitmoji(result, commitType)
	}
	if tmpl != "" {
		result, err = template.ApplyTemplate(tmpl, result)
		if err != nil {
			return "", err
		}
	}
	return result, nil
}
