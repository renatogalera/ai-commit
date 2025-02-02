package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	gogpt "github.com/sashabaranov/go-openai"

	"github.com/renatogalera/ai-commit/pkg/committypes"
	"github.com/renatogalera/ai-commit/pkg/git"
	"github.com/renatogalera/ai-commit/pkg/openai"
	"github.com/renatogalera/ai-commit/pkg/template"
)

// uiState represents the different states of the TUI.
type uiState int

const (
	stateShowCommit uiState = iota
	stateGenerating
	stateCommitting
	stateResult
	stateSelectType
)

// commitResultMsg is returned when a commit attempt finishes.
type commitResultMsg struct {
	err error
}

// regenMsg is returned when an OpenAI regeneration finishes.
type regenMsg struct {
	msg string
	err error
}

// Model defines the state for the TUI.
type Model struct {
	state         uiState
	commitMsg     string
	result        string
	spinner       spinner.Model
	prompt        string
	commitType    string
	template      string
	enableEmoji   bool
	openAIClient  *gogpt.Client
	selectedIndex int
	commitTypes   []string
	regenCount    int
	maxRegens     int
}

// NewUIModel creates a new UI model with the provided parameters.
func NewUIModel(
	commitMsg string,
	prompt string,
	commitType string,
	tmpl string,
	enableEmoji bool,
	client *gogpt.Client,
) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return Model{
		state:         stateShowCommit,
		commitMsg:     commitMsg,
		prompt:        prompt,
		commitType:    commitType,
		template:      tmpl,
		enableEmoji:   enableEmoji,
		openAIClient:  client,
		spinner:       s,
		selectedIndex: 0,
		commitTypes:   committypes.AllTypes(),
		regenCount:    0,
		maxRegens:     3, // Limit the user to 3 regenerations
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
				// User confirms commit
				m.state = stateCommitting
				return m, commitCmd(m.commitMsg)
			case "r":
				// User wants to regenerate
				if m.regenCount >= m.maxRegens {
					// Already at limit
					m.result = fmt.Sprintf("Max regenerations (%d) reached. No more regenerations allowed.", m.maxRegens)
					m.state = stateResult
					return m, nil
				}
				m.state = stateGenerating
				m.spinner = spinner.New()
				m.spinner.Spinner = spinner.Dot
				m.regenCount++
				return m, regenCmd(m.openAIClient, m.prompt, m.commitType, m.template, m.enableEmoji)
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
				m.regenCount++
				return m, regenCmd(m.openAIClient, m.prompt, m.commitType, m.template, m.enableEmoji)
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
		// Update spinner during generating or committing states
		if m.state == stateGenerating || m.state == stateCommitting {
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}
	newModel, cmd := m.updateList(msg)
	return newModel, cmd
}

// updateList is a helper function that could update an internal list if used.
func (m Model) updateList(_ tea.Msg) (tea.Model, tea.Cmd) {
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
		// Use a short context for commit
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := git.CommitChanges(ctx, commitMsg)
		return commitResultMsg{err: err}
	}
}

// regenCmd returns a command to regenerate the commit message using OpenAI.
func regenCmd(
	client *gogpt.Client,
	prompt string,
	commitType string,
	tmpl string,
	enableEmoji bool,
) tea.Cmd {
	return func() tea.Msg {
		msg, err := regenerate(prompt, client, commitType, tmpl, enableEmoji)
		return regenMsg{msg: msg, err: err}
	}
}

// regenerate calls the OpenAI API to generate a new commit message.
func regenerate(
	prompt string,
	client *gogpt.Client,
	commitType string,
	tmpl string,
	enableEmoji bool,
) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := openai.GetChatCompletion(ctx, client, prompt)
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
