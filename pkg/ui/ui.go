// Package ui contains the implementation of the interactive terminal UI (TUI)
// for AI-Commit. This UI lets users view, edit, regenerate, and confirm the AI-
// generated commit message before performing the commit.
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
	"github.com/rs/zerolog/log"

	"github.com/renatogalera/ai-commit/pkg/ai"
	"github.com/renatogalera/ai-commit/pkg/committypes"
	"github.com/renatogalera/ai-commit/pkg/git"
	"github.com/renatogalera/ai-commit/pkg/prompt"
	"github.com/renatogalera/ai-commit/pkg/template"
)

// uiState represents the various states the TUI can be in.
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

type (
	commitResultMsg struct{ err error }
	regenMsg        struct {
		msg string
		err error
	}
	autoQuitMsg struct{}
)

// --- Lipgloss Styles ---

var (
	logoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("62"))

	logoText = `AI-COMMIT TUI
	`

	commitBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1, 2).
			Margin(1, 1)

	sideBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1, 2).
			Margin(1, 1).
			Width(30)

	footerStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252")).
			Padding(0, 1)

	highlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)
)

// Model holds the complete state of the TUI.
type Model struct {
	state       uiState
	commitMsg   string
	result      string
	spinner     spinner.Model
	diff        string
	language    string
	prompt      string
	commitType  string
	template    string
	enableEmoji bool
	aiClient    ai.AIClient

	selectedIndex int
	commitTypes   []string

	regenCount int
	maxRegens  int

	textarea textarea.Model
}

// NewUIModel constructs and returns a new Model with the given parameters.
func NewUIModel(
	commitMsg, diff, language, promptText, commitType, tmpl string,
	enableEmoji bool,
	client ai.AIClient,
) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot

	ta := textarea.New()
	ta.Placeholder = "Edit your commit message or additional prompt here..."
	ta.Prompt = "> "
	ta.SetWidth(50)
	ta.SetHeight(10)
	ta.ShowLineNumbers = false

	// If user didn't explicitly set commitType, try to guess from the commitMsg itself
	if commitType == "" {
		guessed := guessCommitTypeFromMessage(commitMsg)
		if guessed != "" {
			commitType = guessed
		}
	}

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

// NewProgram initializes and returns a new Bubble Tea program using the provided model.
// It also enables an alternative screen (full-screen TUI).
func NewProgram(m Model) *tea.Program {
	return tea.NewProgram(m, tea.WithAltScreen())
}

// Init is part of the Bubble Tea interface and returns the initial command.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and events, updating the TUI state accordingly.
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
			cmd = tcmd
			switch msg.String() {
			case "esc":
				m.state = stateShowCommit
				return m, cmd
			case "ctrl+s":
				m.commitMsg = m.textarea.Value()
				m.state = stateShowCommit
				return m, cmd
			}
			return m, cmd

		case stateEditingPrompt:
			var tcmd tea.Cmd
			m.textarea, tcmd = m.textarea.Update(msg)
			cmd = tcmd
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
			return m, cmd
		}

	case regenMsg:
		log.Debug().Msgf("regenMsg received with commit message: %q", msg.msg)
		if msg.err != nil {
			m.result = fmt.Sprintf("Error: %v", msg.err)
			m.state = stateResult
			return m, autoQuitCmd()
		}
		// Update commit message
		m.commitMsg = msg.msg
		// If still no commitType, guess from the newly regenerated message
		if m.commitType == "" {
			if guessed := guessCommitTypeFromMessage(m.commitMsg); guessed != "" {
				m.commitType = guessed
			}
		}
		m.state = stateShowCommit
		return m, nil

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

// View renders the TUI for each state.
func (m Model) View() string {
	switch m.state {
	case stateShowCommit:
		return m.viewShowCommit()
	case stateGenerating:
		return m.viewGenerating()
	case stateCommitting:
		return m.viewCommitting()
	case stateResult:
		return m.viewResult()
	case stateSelectType:
		return m.viewSelectType()
	case stateEditing:
		return m.viewEditing("Editing commit message (ESC to cancel, Ctrl+S to save):")
	case stateEditingPrompt:
		return m.viewEditing("Editing prompt text (ESC to cancel, Ctrl+S to apply):")
	default:
		return "Unknown state."
	}
}

// viewShowCommit ...
func (m Model) viewShowCommit() string {
	header := renderLogo()
	footer := m.renderFooter()

	content := commitBoxStyle.Render(m.commitMsg)
	side := m.renderSideInfo()

	mainCols := lipgloss.JoinHorizontal(
		lipgloss.Top,
		content,
		side,
	)

	ui := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		mainCols,
		footer,
	)
	return ui
}

func (m Model) viewGenerating() string {
	header := renderLogo()
	body := fmt.Sprintf("Generating commit message...\n\n%s", m.spinner.View())
	footer := m.renderFooter()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		body,
		footer,
	)
}

func (m Model) viewCommitting() string {
	header := renderLogo()
	body := fmt.Sprintf("Committing...\n\n%s", m.spinner.View())
	footer := m.renderFooter()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		body,
		footer,
	)
}

func (m Model) viewResult() string {
	header := renderLogo()
	body := lipgloss.NewStyle().Margin(1, 2).Render(m.result)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		body,
	)
}

func (m Model) viewSelectType() string {
	header := renderLogo()

	var b strings.Builder
	b.WriteString("Select commit type:\n\n")
	for i, ct := range m.commitTypes {
		cursor := " "
		if i == m.selectedIndex {
			cursor = highlightStyle.Render(">")
		}
		b.WriteString(fmt.Sprintf("%s %s\n", cursor, ct))
	}

	footer := lipgloss.NewStyle().Margin(1, 0).Render(
		"Use ↑/↓ (or j/k) to navigate, Enter to select, 'q' to cancel.\n",
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		b.String(),
		footer,
	)
}

func (m Model) viewEditing(title string) string {
	header := renderLogo()

	body := lipgloss.NewStyle().Margin(1, 2).Render(
		fmt.Sprintf("%s\n\n%s", title, m.textarea.View()),
	)

	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

// --- Helpers ---

func renderLogo() string {
	return logoStyle.Render(logoText)
}

func (m Model) renderSideInfo() string {
	info := []string{
		highlightStyle.Render("Commit Type: ") + m.commitType,
		highlightStyle.Render("Regens Left: ") + fmt.Sprintf("%d/%d", m.maxRegens-m.regenCount, m.maxRegens),
		highlightStyle.Render("Language: ") + m.language,
	}
	return sideBoxStyle.Render(strings.Join(info, "\n\n"))
}

func (m Model) renderFooter() string {
	helpText := "Press 'y' to commit, 'r' to regenerate, 'e' to edit, 't' to change type, 'p' to add prompt text, 'q' to quit."
	return footerStyle.Render(helpText)
}

// commitCmd triggers the Git commit operation with a given commit message.
func commitCmd(commitMsg string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := git.CommitChanges(ctx, commitMsg)
		return commitResultMsg{err: err}
	}
}

// regenCmd calls the AI to regenerate the commit message and returns a regenMsg.
func regenCmd(client ai.AIClient, prompt, commitType, tmpl string, enableEmoji bool) tea.Cmd {
	return func() tea.Msg {
		msg, err := regenerate(prompt, client, commitType, tmpl, enableEmoji)
		return regenMsg{msg: msg, err: err}
	}
}

// regenerate calls the AI client to generate a new commit message, then sanitizes
// and applies any template or emoji rules.
func regenerate(prompt string, client ai.AIClient, commitType, tmpl string, enableEmoji bool) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	log.Debug().Msg("Calling GetCommitMessage on AI client")
	result, err := client.GetCommitMessage(ctx, prompt)
	if err != nil {
		log.Error().Err(err).Msg("GetCommitMessage returned an error")
		return "", err
	}
	log.Debug().Msg("Received response from AI client")

	// Sanitize the AI output
	result = ai.SanitizeResponse(result, commitType)

	// Possibly add an emoji if requested
	if enableEmoji {
		result = git.AddGitmoji(result, commitType)
	}

	// Apply template if user specified one
	if tmpl != "" {
		result, err = template.ApplyTemplate(tmpl, result)
		if err != nil {
			return "", err
		}
	}

	return result, nil
}

// autoQuitCmd issues a command to automatically quit after a 2-second delay.
func autoQuitCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return autoQuitMsg{}
	})
}

// guessCommitTypeFromMessage tries to detect a commit type from the message
// if the user hasn't specified one.
//
// CHANGED: "feat" is checked before "fix" so that if the text has both
// "feat" and "fix", "feat" wins out.
func guessCommitTypeFromMessage(msg string) string {
	lower := strings.ToLower(msg)

	// Put "feat" above "fix" so it's matched first if both strings appear
	switch {
	case strings.Contains(lower, "feat"), strings.Contains(lower, "add"),
		strings.Contains(lower, "create"), strings.Contains(lower, "introduce"):
		return "feat"
	case strings.Contains(lower, "fix"):
		return "fix"
	case strings.Contains(lower, "doc"):
		return "docs"
	case strings.Contains(lower, "refactor"):
		return "refactor"
	case strings.Contains(lower, "test"):
		return "test"
	case strings.Contains(lower, "perf"):
		return "perf"
	case strings.Contains(lower, "build"):
		return "build"
	case strings.Contains(lower, "ci"):
		return "ci"
	case strings.Contains(lower, "chore"):
		return "chore"
	default:
		return ""
	}
}
