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

// uiState represents the different states of the TUI.
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

	logoText = `AI-COMMIT`

	// Where the commit message is shown
	commitBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1, 2).
			Margin(1, 1)

	// A smaller style for info lines that are not as important
	infoLineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Margin(0, 1).
			Italic(true)

	highlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)

	diffStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
)

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

	// styleReview holds optional suggestions from AI for commit style:
	styleReview string
	
	// Terminal dimensions
	width  int
	height int
}

// NewUIModel creates a new TUI model.
func NewUIModel(
	commitMsg, diff, language, promptText, commitType, tmpl string,
	styleReviewSuggestions string,
	enableEmoji bool,
	client ai.AIClient,
) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot

	ta := textarea.New()
	ta.Placeholder = "Edit your commit message or additional prompt here..."
	ta.Prompt = "> "
	// Initial dimensions will be set by WindowSizeMsg
	ta.SetWidth(80)
	ta.SetHeight(10)
	ta.ShowLineNumbers = false

	if commitType == "" {
		if guessed := committypes.GuessCommitType(commitMsg); guessed != "" {
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
		commitTypes:   committypes.GetAllTypes(),
		regenCount:    0,
		maxRegens:     3,
		textarea:      ta,
		help:          help.New(),

		styleReview: styleReviewSuggestions,
	}
}

// NewProgram creates a new Bubble Tea program with the given model.
func NewProgram(m Model) *tea.Program {
	return tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
}

// Init is the Bubble Tea initialization command.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
	)
}

// --- UPDATE ------------------------------------------------------------------

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		// Update textarea dimensions based on terminal size
		textareaWidth := min(m.width-4, 80) // Max width of 80 chars
		textareaHeight := min(m.height-10, 20) // Leave room for UI elements
		m.textarea.SetWidth(textareaWidth)
		m.textarea.SetHeight(textareaHeight)
		
		return m, nil

	case tea.KeyMsg:
		// Handle editing states first to prevent key conflicts
		if m.state == stateEditing || m.state == stateEditingPrompt {
			var tcmd tea.Cmd
			m.textarea, tcmd = m.textarea.Update(msg)
			
			// Only handle specific control keys in editing modes
			switch msg.String() {
			case "ctrl+s":
				if m.state == stateEditing {
					m.commitMsg = m.textarea.Value()
					m.state = stateShowCommit
				} else if m.state == stateEditingPrompt {
					userPrompt := m.textarea.Value()
					m.state = stateGenerating
					m.spinner = spinner.New()
					m.spinner.Spinner = spinner.Dot
					m.regenCount++
					m.prompt = prompt.BuildCommitPrompt(m.diff, m.language, m.commitType, userPrompt, "")
					return m, regenCmd(m.aiClient, m.prompt, m.commitType, m.template, m.enableEmoji)
				}
			case "esc":
				m.state = stateShowCommit
			}
			return m, tcmd
		}
		
		// Handle global keys for non-editing states
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
				// Rebuild the prompt with the newly selected commit type
				m.prompt = prompt.BuildCommitPrompt(m.diff, m.language, m.commitType, "", "")
				return m, regenCmd(m.aiClient, m.prompt, m.commitType, m.template, m.enableEmoji)
			case "esc", "q":
				m.state = stateShowCommit
				return m, nil
			}

		// These cases are now handled at the beginning of tea.KeyMsg

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

	case viewDiffMsg:
		m.state = stateShowDiff
		return m, nil

	case spinner.TickMsg:
		// Keep spinner going while in generating or committing
		if m.state == stateGenerating || m.state == stateCommitting {
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}
	return m, cmd
}

// --- VIEWS -------------------------------------------------------------------

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
		return m.viewDiff()
	default:
		return "Unknown state."
	}
}

// viewShowCommit has been updated to present the info line in smaller text
// above the main commit message box, in a single vertical layout.
func (m Model) viewShowCommit() string {
	// 1) The TUI Banner
	header := logoStyle.Render(logoText)

	// 2) A subtle info line
	infoText := fmt.Sprintf("Type: %s | Regens Left: %d/%d | Language: %s",
		m.commitType, (m.maxRegens - m.regenCount), m.maxRegens, m.language)
	infoLine := infoLineStyle.Render(infoText)

	// 3) The commit box - adjust width based on terminal size
	boxWidth := min(m.width-4, 100) // Leave some margin, max 100 chars
	commitBoxStyleAdaptive := commitBoxStyle.Width(boxWidth)
	content := commitBoxStyleAdaptive.Render(m.commitMsg)

	// 4) If styleReview is not trivial or "no issues found", show it
	styleReviewSection := ""
	if trimmed := strings.TrimSpace(m.styleReview); trimmed != "" &&
		!strings.Contains(strings.ToLower(trimmed), "no issues found") {
		boxWidth := min(m.width-4, 100) // Same width as commit box
		styleReviewSection = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("204")).
			Padding(1, 2).
			Margin(1, 1).
			Width(boxWidth).
			Render("Style Review Suggestions:\n\n" + trimmed)
	}

	// 5) The help view
	helpView := m.help.View(m)

	// Merge everything in one vertical column
	builder := strings.Builder{}
	builder.WriteString(header + "\n\n")
	builder.WriteString(infoLine + "\n")
	builder.WriteString(content + "\n")

	if styleReviewSection != "" {
		builder.WriteString(styleReviewSection + "\n")
	}

	builder.WriteString(helpView + "\n")
	return builder.String()
}

func (m Model) viewGenerating() string {
	header := logoStyle.Render(logoText)
	body := fmt.Sprintf("Generating commit message... (Attempt %d/%d)\n\n%s", m.regenCount, m.maxRegens, m.spinner.View())
	helpView := m.help.View(m)

	return lipgloss.JoinVertical(lipgloss.Left, header, body, helpView)
}

func (m Model) viewCommitting() string {
	header := logoStyle.Render(logoText)
	body := fmt.Sprintf("Committing...\n\n%s", m.spinner.View())
	helpView := m.help.View(m)

	return lipgloss.JoinVertical(lipgloss.Left, header, body, helpView)
}

func (m Model) viewResult() string {
	header := logoStyle.Render(logoText)
	body := lipgloss.NewStyle().Margin(1, 2).Render(m.result)
	helpView := m.help.View(m)

	return lipgloss.JoinVertical(lipgloss.Left, header, body, helpView)
}

func (m Model) viewSelectType() string {
	header := logoStyle.Render(logoText)
	var b strings.Builder
	b.WriteString("Select commit type:\n\n")
	for i, ct := range m.commitTypes {
		cursor := " "
		if i == m.selectedIndex {
			cursor = highlightStyle.Render(">")
		}
		b.WriteString(fmt.Sprintf("%s %s\n", cursor, ct))
	}
	b.WriteString("\nUse up/down (or j/k) to navigate, enter to select, 'q' to cancel.\n")

	helpView := m.help.View(m)
	return lipgloss.JoinVertical(lipgloss.Left, header, b.String(), helpView)
}

func (m Model) viewEditing(title string) string {
	header := logoStyle.Render(logoText)
	body := lipgloss.NewStyle().Margin(1, 2).Render(
		fmt.Sprintf("%s\n\n%s", title, m.textarea.View()),
	)
	helpView := m.help.View(m)

	return lipgloss.JoinVertical(lipgloss.Left, header, body, helpView)
}

func (m Model) viewDiff() string {
	header := logoStyle.Render(logoText)
	diffTextView := diffStyle.Render(m.diff)
	body := lipgloss.NewStyle().Margin(1, 2).Render(
		fmt.Sprintf("Git Diff:\n\n%s\n\nPress ESC/q to return.", diffTextView),
	)
	helpView := m.help.View(m)

	return lipgloss.JoinVertical(lipgloss.Left, header, body, helpView)
}

// --- COMMANDS ----------------------------------------------------------------

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

	return strings.TrimSpace(result), nil
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

// -------------------------------------------------------------------------------------
// Added methods so Model implements help.KeyMap (for m.help.View(m)).
// -------------------------------------------------------------------------------------

func (m Model) ShortHelp() []key.Binding {
	return []key.Binding{
		keyMap.Commit,
		keyMap.Regenerate,
		keyMap.Edit,
		keyMap.TypeSelect,
		keyMap.PromptEdit,
		keyMap.ViewDiff,
		keyMap.Help,
		keyMap.Quit,
		keyMap.Enter,
	}
}

func (m Model) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		m.ShortHelp(),
	}
}

// GetAIClient returns the AI client stored in the UI model.
func (m Model) GetAIClient() ai.AIClient {
	return m.aiClient
}

// GetCommitMsg returns the commit message stored in the UI model.
func (m Model) GetCommitMsg() string {
	return m.commitMsg
}
