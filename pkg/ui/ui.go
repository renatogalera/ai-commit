package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
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

type uiState int

const (
	stateShowCommit uiState = iota
	stateGenerating
	stateCommitting
	stateResult
	stateSelectType
	stateEditing
	stateEditingPrompt
	stateShowDiff
)

type (
	commitResultMsg struct{ err error }
	regenMsg        struct {
		msg string
		err error
	}
	autoQuitMsg struct{}
	viewDiffMsg struct{}
)

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

	highlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)

	diffStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
)

// --- Keybindings ---
type keys struct {
	Commit     key.Binding
	Regenerate key.Binding
	Edit       key.Binding
	TypeSelect key.Binding
	PromptEdit key.Binding
	Quit       key.Binding
	ViewDiff   key.Binding
	Help       key.Binding
	Enter      key.Binding
}

var keyMap = keys{
	Commit: key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("y", "commit"),
	),
	Regenerate: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "regenerate"),
	),
	Edit: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "edit message"),
	),
	TypeSelect: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "change type"),
	),
	PromptEdit: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "edit prompt"),
	),
	ViewDiff: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "view diff"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c", "esc"),
		key.WithHelp("q", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "commit"),
	),
}

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
	help     help.Model
}

func (m Model) GetCommitMsg() string {
	return m.commitMsg
}

func (m Model) GetAIClient() ai.AIClient {
	return m.aiClient
}

func (m Model) ShortHelp() []key.Binding {
	return []key.Binding{keyMap.Help, keyMap.Quit, keyMap.Commit, keyMap.Regenerate} // Basic bindings
}

func (m Model) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{keyMap.Commit, keyMap.Regenerate, keyMap.Edit, keyMap.TypeSelect, keyMap.PromptEdit, keyMap.ViewDiff}, // More actions
		{keyMap.Help, keyMap.Quit},
	}
}

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

	if commitType == "" {
		guessed := committypes.GuessCommitType(commitMsg)
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
		help:          help.New(),
	}
}

func NewProgram(m Model) *tea.Program {
	return tea.NewProgram(m, tea.WithAltScreen())
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, keyMap.Quit) {
			return m, tea.Quit
		}
		if key.Matches(msg, keyMap.Help) {
			m.help.ShowAll = !m.help.ShowAll
			return m, nil
		}

		switch m.state {
		case stateShowCommit:
			if key.Matches(msg, keyMap.Commit, keyMap.Enter) {
				m.state = stateCommitting
				return m, commitCmd(m.commitMsg)
			}
			if key.Matches(msg, keyMap.Regenerate) {
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
			}
			if key.Matches(msg, keyMap.TypeSelect) {
				m.state = stateSelectType
				return m, nil
			}
			if key.Matches(msg, keyMap.Edit) {
				m.state = stateEditing
				m.textarea.SetValue(m.commitMsg)
				m.textarea.Focus()
				return m, nil
			}
			if key.Matches(msg, keyMap.PromptEdit) {
				m.state = stateEditingPrompt
				m.textarea.SetValue("")
				m.textarea.Focus()
				return m, nil
			}
			if key.Matches(msg, keyMap.ViewDiff) {
				m.state = stateShowDiff
				return m, viewDiffCmd(m.diff)
			}

		case stateSelectType:
			switch msg.String() {
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
				m.prompt = prompt.BuildCommitPrompt(m.diff, m.language, m.commitType, "", "")
				return m, regenCmd(m.aiClient, m.prompt, m.commitType, m.template, m.enableEmoji)
			}

		case stateEditing:
			var tcmd tea.Cmd
			m.textarea, tcmd = m.textarea.Update(msg)
			cmd = tcmd
			if key.Matches(msg, keyMap.Quit) {
				m.state = stateShowCommit
				return m, cmd
			}
			if msg.String() == "ctrl+s" {
				m.commitMsg = m.textarea.Value()
				m.state = stateShowCommit
				return m, cmd
			}
			return m, cmd

		case stateEditingPrompt:
			var tcmd tea.Cmd
			m.textarea, tcmd = m.textarea.Update(msg)
			cmd = tcmd
			if key.Matches(msg, keyMap.Quit) {
				m.state = stateShowCommit
				return m, cmd
			}
			if msg.String() == "ctrl+s" {
				userPrompt := m.textarea.Value()
				m.state = stateGenerating
				m.spinner = spinner.New()
				m.spinner.Spinner = spinner.Dot
				m.regenCount++
				m.prompt = prompt.BuildCommitPrompt(m.diff, m.language, m.commitType, userPrompt, "")
				return m, regenCmd(m.aiClient, m.prompt, m.commitType, m.template, m.enableEmoji)
			}
			return m, cmd

		case stateShowDiff:
			if key.Matches(msg, keyMap.Quit) {
				m.state = stateShowCommit
				return m, nil
			}
		}

	case regenMsg:
		log.Debug().Msgf("regenMsg received with commit message: %q", msg.msg)
		if msg.err != nil {
			m.result = fmt.Sprintf("Error: %v", msg.err)
			m.state = stateResult
			return m, autoQuitCmd()
		}
		m.commitMsg = msg.msg
		if m.commitType == "" {
			if guessed := committypes.GuessCommitType(m.commitMsg); guessed != "" {
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

	case viewDiffMsg:
		m.state = stateShowDiff
		return m, nil

	}
	return m, cmd
}

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
		return m.viewEditing("Editing commit message (Ctrl+S to save, ESC to cancel):")
	case stateEditingPrompt:
		return m.viewEditing("Editing prompt text (Ctrl+S to apply, ESC to cancel):")
	case stateShowDiff:
		return m.viewDiff() // Render diff view
	default:
		return "Unknown state."
	}
}

func (m Model) viewShowCommit() string {
	header := renderLogo()
	footer := m.renderFooter()
	helpView := m.help.View(m) // Help view

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
		helpView,
	)
	return ui
}

func (m Model) viewGenerating() string {
	header := renderLogo()
	body := fmt.Sprintf("Generating commit message... (Attempt %d/%d)\n\n%s", m.regenCount, m.maxRegens, m.spinner.View())
	footer := m.renderFooter()
	helpView := m.help.View(m)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		body,
		footer,
		helpView,
	)
}

func (m Model) viewCommitting() string {
	header := renderLogo()
	body := fmt.Sprintf("Committing...\n\n%s", m.spinner.View())
	footer := m.renderFooter()
	helpView := m.help.View(m)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		body,
		footer,
		helpView,
	)
}

func (m Model) viewResult() string {
	header := renderLogo()
	body := lipgloss.NewStyle().Margin(1, 2).Render(m.result)
	helpView := m.help.View(m)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		body,
		helpView,
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
		"Use ↑/↓ (or j/k) to navigate, Enter to select, ESC/q to cancel.\n",
	)
	helpView := m.help.View(m)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		b.String(),
		footer,
		helpView,
	)
}

func (m Model) viewEditing(title string) string {
	header := renderLogo()

	body := lipgloss.NewStyle().Margin(1, 2).Render(
		fmt.Sprintf("%s\n\n%s", title, m.textarea.View()),
	)
	helpView := m.help.View(m)

	return lipgloss.JoinVertical(lipgloss.Left, header, body, helpView)
}

func (m Model) viewDiff() string {
	header := renderLogo()
	diffTextView := diffStyle.Render(m.diff)

	body := lipgloss.NewStyle().Margin(1, 2).Render(
		fmt.Sprintf("Git Diff:\n\n%s\n\nPress ESC/q to return.", diffTextView),
	)
	helpView := m.help.View(m)

	return lipgloss.JoinVertical(lipgloss.Left, header, body, helpView)
}

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

func regenCmd(client ai.AIClient, prompt, commitType, tmpl string, enableEmoji bool) tea.Cmd {
	return func() tea.Msg {
		msg, err := regenerate(prompt, client, commitType, tmpl, enableEmoji)
		return regenMsg{msg: msg, err: err}
	}
}

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

	result = client.SanitizeResponse(result, commitType)

	if commitType != "" {
		result = git.PrependCommitType(result, commitType, enableEmoji)
	}

	if tmpl != "" {
		result, err = template.ApplyTemplate(tmpl, result)
		if err != nil {
			return "", err
		}
	}

	return result, nil
}

func autoQuitCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return autoQuitMsg{}
	})
}

func viewDiffCmd(diff string) tea.Cmd {
	return func() tea.Msg {
		return viewDiffMsg{}
	}
}
