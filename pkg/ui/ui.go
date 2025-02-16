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
	"github.com/renatogalera/ai-commit/pkg/prompt"
	"github.com/renatogalera/ai-commit/pkg/template"
)

// Possible states of our TUI.
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

// Internal message types for bubbletea updates.
type (
	commitResultMsg struct{ err error }
	regenMsg        struct {
		msg string
		err error
	}
	autoQuitMsg struct{}
)

// Here we define some lipgloss styles for our TUI elements.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")) // Pinkish

	commitBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")). // A soft blue
			Padding(1, 2).
			Margin(1, 0)

	helpBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1, 2).
			Margin(1, 0)

	highlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")). // Pink
			Bold(true)
)

// Model holds the data needed by our TUI.
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

// renderHeader creates a decorative ASCII header at the top of the TUI.
func renderHeader() string {
	return titleStyle.Render(`
  ____  _       ____                           
 / ___|| |_ __ / ___|  ___ _ ____   _____ _ __ 
 \___ \| | '__| |  _  / _ \ '__\ \ / / _ \ '__|
  ___) | | |  | |_| |  __/ |   \ V /  __/ |     
 |____/|_|_|   \____|\___|_|    \_/ \___|_|     
   
          AI-Commit TUI
`)
}

// NewUIModel constructs the Bubble Tea model for the main commit flow.
func NewUIModel(commitMsg, diff, language, promptText, commitType, tmpl string, enableEmoji bool, client ai.AIClient) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot

	ta := textarea.New()
	ta.Placeholder = "Edit your commit message or additional prompt here..."
	ta.Prompt = "> "
	ta.SetWidth(50)
	ta.SetHeight(10)
	ta.ShowLineNumbers = false

	return Model{
		state:         stateShowCommit,
		commitMsg:     commitMsg,
		diff:          diff,
		language:      language,
		prompt:        promptText,
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

// NewProgram initializes a new Bubble Tea program.
func NewProgram(m Model) *tea.Program {
	return tea.NewProgram(m, tea.WithAltScreen())
}

// Init is part of the Bubble Tea interface (no-op here).
func (m Model) Init() tea.Cmd {
	return nil
}

// Update processes events and updates the TUI state accordingly.
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
					m.result = fmt.Sprintf("Maximum regenerations (%d) reached.", m.maxRegens)
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
			// Once we have a result (commit done or error), auto-quit after a moment
			return m, autoQuitCmd()
		}

	case regenMsg:
		if msg.err != nil {
			m.result = fmt.Sprintf("Error: %v", msg.err)
			m.state = stateResult
			return m, autoQuitCmd()
		}
		m.commitMsg = msg.msg
		m.state = stateShowCommit

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

	// Update the textarea for states that might need it
	newTA, tcmd := m.textarea.Update(msg)
	m.textarea = newTA
	return m, tcmd
}

// View renders the TUI for each state.
func (m Model) View() string {
	switch m.state {
	case stateShowCommit:
		// Render a fancy header and two boxes (commit message + help instructions)
		header := renderHeader()
		commitBox := commitBoxStyle.Render(m.commitMsg)

		helpText := highlightStyle.Render(
			"Press 'y' to commit, 'r' to regenerate,\n" +
				"'e' to edit, 't' to change type,\n" +
				"'p' to add prompt text, 'q' to quit.",
		)
		helpBox := helpBoxStyle.Render(helpText)

		// Join everything vertically
		return lipgloss.JoinVertical(lipgloss.Left, header, commitBox, helpBox)

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
		b.WriteString("\nUse ↑/↓ (or j/k) to navigate, Enter to select, 'q' to cancel.\n")
		return b.String()

	case stateEditing:
		return fmt.Sprintf("Editing commit message (ESC to cancel, Ctrl+S to save):\n\n%s", m.textarea.View())

	case stateEditingPrompt:
		return fmt.Sprintf("Editing prompt text (ESC to cancel, Ctrl+S to apply):\n\n%s", m.textarea.View())
	}
	return ""
}

// commitCmd triggers the actual commit via Git.
func commitCmd(commitMsg string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := git.CommitChanges(ctx, commitMsg)
		return commitResultMsg{err: err}
	}
}

// regenCmd calls the AI to regenerate a commit message.
func regenCmd(client ai.AIClient, prompt, commitType, tmpl string, enableEmoji bool) tea.Cmd {
	return func() tea.Msg {
		msg, err := regenerate(prompt, client, commitType, tmpl, enableEmoji)
		return regenMsg{msg: msg, err: err}
	}
}

// regenerate is a helper to fetch a new commit message from the AI, sanitize, and apply template/emoji.
func regenerate(prompt string, client ai.AIClient, commitType, tmpl string, enableEmoji bool) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := client.GetCommitMessage(ctx, prompt)
	if err != nil {
		return "", err
	}
	result = ai.SanitizeResponse(result, commitType)
	if enableEmoji {
		result = git.AddGitmoji(result, commitType)
	}
	if tmpl != "" {
		result, err = template.ApplyTemplate(tmpl, result)
		if err != nil {
			return "", err
		}
	}
	return result, nil
}

// autoQuitCmd is used to exit automatically after a short delay, e.g., after showing a final result.
func autoQuitCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return autoQuitMsg{}
	})
}
