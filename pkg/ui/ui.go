package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
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
	streamStartedMsg struct {
		deltaCh <-chan string
		doneCh  <-chan error
	}
	streamDeltaMsg struct{ delta string }
	streamDoneMsg  struct{ err error }
	autoQuitMsg    struct{}
	viewDiffMsg    struct{}
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

	// Error box style
	errorBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("196")).
			Foreground(lipgloss.Color("196")).
			Bold(true).
			Padding(1, 2).
			Margin(1, 1)
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

	// streaming support
	startStreaming bool
	streamDeltaCh  <-chan string
	streamDoneCh   <-chan error

	// animation
	progress     progress.Model
	progValue    float64
	dotFrame     int
	revealActive bool
	displayedMsg string

	selectedIndex int
	commitTypes   []string

	regenCount int
	maxRegens  int

	textarea textarea.Model
	help     help.Model

	// styleReview holds optional suggestions from AI for commit style:
	styleReview string
	// last error message to display prominently
	errMsg string

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
	startStreaming bool,
) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot

	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)

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
		progress:      p,
		selectedIndex: 0,
		commitTypes:   committypes.GetAllTypes(),
		regenCount:    0,
		maxRegens:     3,
		textarea:      ta,
		help:          help.New(),

		styleReview:   styleReviewSuggestions,
		startStreaming: startStreaming,
		errMsg:         "",
		progValue:      0,
		dotFrame:       0,
		revealActive:   false,
		displayedMsg:   commitMsg,
	}
}

// NewProgram creates a new Bubble Tea program with the given model.
func NewProgram(m Model) *tea.Program {
	return tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
}

// Init is the Bubble Tea initialization command.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tea.EnterAltScreen}
	if m.startStreaming {
		// kick off streaming immediately
		cmds = append(cmds, startStreamCmd(m.aiClient, m.prompt))
	}
	// initialize progress bar animation frames
	if initCmd := m.progress.Init(); initCmd != nil {
		cmds = append(cmds, initCmd)
	}
	return tea.Batch(cmds...)
}

// --- UPDATE ------------------------------------------------------------------

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// Always let the progress bar consume relevant messages first.
	if p, pcmd := m.progress.Update(msg); pcmd != nil {
		m.progress = p.(progress.Model)
		cmds = append(cmds, pcmd)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update textarea dimensions based on terminal size
		textareaWidth := min(m.width-4, 80)    // Max width of 80 chars
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
				m.errMsg = ""
				// Ensure spinner animates while committing
				m.spinner = spinner.New()
				m.spinner.Spinner = spinner.Dot
				return m, tea.Batch(m.spinner.Tick, commitCmd(m.commitMsg))
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
				m.errMsg = ""
				return m, tea.Batch(m.spinner.Tick,
					regenCmd(m.aiClient, m.prompt, m.commitType, m.template, m.enableEmoji))
			}
			if key.Matches(msg, keyMap.TypeSelect) {
				m.state = stateSelectType
				m.errMsg = ""
				return m, nil
			}
			if key.Matches(msg, keyMap.Edit) {
				m.state = stateEditing
				m.errMsg = ""
				m.textarea.SetValue(m.commitMsg)
				m.textarea.Focus()
				return m, nil
			}
			if key.Matches(msg, keyMap.PromptEdit) {
				m.state = stateEditingPrompt
				m.errMsg = ""
				m.textarea.SetValue("")
				m.textarea.Focus()
				return m, nil
			}
			if key.Matches(msg, keyMap.ViewDiff) {
				m.state = stateShowDiff
				m.errMsg = ""
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
				return m, tea.Batch(m.spinner.Tick,
					regenCmd(m.aiClient, m.prompt, m.commitType, m.template, m.enableEmoji))
			case "esc", "q":
				m.state = stateShowCommit
				return m, nil
			}

		case stateShowDiff:
			if key.Matches(msg, keyMap.Quit) {
				m.state = stateShowCommit
				return m, nil
			}
		}

	case regenMsg:
		log.Debug().Msgf("regenMsg received with commit message: %q", msg.msg)
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("AI error: %v", msg.err)
			m.state = stateShowCommit
			return m, nil
		}
		m.commitMsg = msg.msg
		if m.commitType == "" {
			if guessed := committypes.GuessCommitType(m.commitMsg); guessed != "" {
				m.commitType = guessed
			}
		}
		// Animate reveal for non-streaming providers
		m.revealActive = true
		m.displayedMsg = ""
		m.state = stateGenerating
		m.spinner = spinner.New()
		m.spinner.Spinner = spinner.Dot
		return m, m.spinner.Tick

	case commitResultMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("Commit failed: %v", msg.err)
			m.state = stateShowCommit
			return m, nil
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

	case streamStartedMsg:
		// IMPORTANT: start spinner ticks so we get spinner.TickMsg,
		// which we use as the heartbeat to advance the progress bar.
		m.state = stateGenerating
		m.spinner = spinner.New()
		m.spinner.Spinner = spinner.Dot
		m.streamDeltaCh = msg.deltaCh
		m.streamDoneCh = msg.doneCh
		m.errMsg = ""
		return m, tea.Batch(
			m.spinner.Tick,                  // <— start ticks here (fix)
			readDeltaCmd(m.streamDeltaCh),
			waitDoneCmd(m.streamDoneCh),
		)

	case streamDeltaMsg:
		m.commitMsg += msg.delta
		// keep waiting for more deltas
		return m, readDeltaCmd(m.streamDeltaCh)

	case streamDoneMsg:
		// finalize message: sanitize, prepend type, apply template
		final := m.commitMsg
		final = m.aiClient.SanitizeResponse(final, m.commitType)
		if m.commitType != "" {
			final = git.PrependCommitType(final, m.commitType, m.enableEmoji)
		}
		if m.template != "" {
			if res, err := template.ApplyTemplate(m.template, final); err == nil {
				final = res
			}
		}
		m.commitMsg = strings.TrimSpace(final)
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("AI streaming error: %v", msg.err)
		}
		m.state = stateShowCommit
		return m, nil

	case spinner.TickMsg:
		// Keep spinner and animations going while in generating or committing
		if m.state == stateGenerating || m.state == stateCommitting {
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
			// Indefinite progress and typing indicator heartbeat
			m.progValue += 0.03
			if m.progValue > 1.2 {
				m.progValue = 0
			}
			m.dotFrame = (m.dotFrame + 1) % 4
			// Typewriter reveal for non-streaming
			if m.revealActive {
				dr := []rune(m.displayedMsg)
				tr := []rune(m.commitMsg)
				if len(dr) < len(tr) {
					step := 3
					end := len(dr) + step
					if end > len(tr) {
						end = len(tr)
					}
					m.displayedMsg = string(tr[:end])
				} else {
					m.revealActive = false
					m.state = stateShowCommit
				}
			}
			// Update progress bar percent; progress will consume its own messages.
			cmds = append(cmds, m.progress.SetPercent(m.progValue))
			return m, tea.Batch(cmds...)
		}
	}
	return m, tea.Batch(cmds...)
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

	// 3) Optional error box
	errSection := ""
	if strings.TrimSpace(m.errMsg) != "" {
		boxWidth := min(m.width-4, 100)
		errSection = errorBoxStyle.Width(boxWidth).Render(m.errMsg)
	}

	// 4) The commit box - adjust width based on terminal size
	boxWidth := min(m.width-4, 100) // Leave some margin, max 100 chars
	commitBoxStyleAdaptive := commitBoxStyle.Width(boxWidth)
	content := commitBoxStyleAdaptive.Render(m.commitMsg)

	// 5) If styleReview is not trivial or "no issues found", show it
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

	// 6) The help view
	helpView := m.help.View(m)

	// Merge everything in one vertical column
	builder := strings.Builder{}
	builder.WriteString(header + "\n\n")
	builder.WriteString(infoLine + "\n")
	if errSection != "" {
		builder.WriteString(errSection + "\n")
	}
	builder.WriteString(content + "\n")

	if styleReviewSection != "" {
		builder.WriteString(styleReviewSection + "\n")
	}

	builder.WriteString(helpView + "\n")
	return builder.String()
}

func (m Model) viewGenerating() string {
	header := logoStyle.Render(logoText)
	// Show partial output while spinning and any error
	boxWidth := min(m.width-4, 100)
	commitBoxStyleAdaptive := commitBoxStyle.Width(boxWidth)
	showText := m.commitMsg
	if m.revealActive {
		showText = m.displayedMsg
	}
	partial := commitBoxStyleAdaptive.Render(showText)
	errSection := ""
	if strings.TrimSpace(m.errMsg) != "" {
		errSection = errorBoxStyle.Width(boxWidth).Render(m.errMsg) + "\n\n"
	}
	// Fancy typing indicator and progress bar
	dots := strings.Repeat(".", m.dotFrame)
	genLine := fmt.Sprintf("Generating commit message%s", dots)
	progView := m.progress.View()
	body := fmt.Sprintf("%s\n%s\n\n%s%s",
		genLine, progView, errSection, partial)
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

// commitCmd executes "git commit" with a timeout and returns the result as a msg.
func commitCmd(commitMsg string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := git.CommitChanges(ctx, commitMsg)
		return commitResultMsg{err: err}
	}
}

// regenCmd calls the AI client to (re)generate a commit message.
// If the client supports streaming, it wires channels and returns streamStartedMsg.
func regenCmd(client ai.AIClient, prompt, commitType, tmpl string, enableEmoji bool) tea.Cmd {
	return func() tea.Msg {
		// Try streaming if available
		if sc, ok := client.(ai.StreamingAIClient); ok {
			deltaCh := make(chan string, 64)
			doneCh := make(chan error, 1)
			go func() {
				_, err := sc.StreamCommitMessage(context.Background(), prompt, func(d string) {
					deltaCh <- d
				})
				close(deltaCh)
				doneCh <- err
				close(doneCh)
			}()
			return streamStartedMsg{deltaCh: deltaCh, doneCh: doneCh}
		}
		msg, err := regenerate(prompt, client, commitType, tmpl, enableEmoji)
		return regenMsg{msg: msg, err: err}
	}
}

// startStreamCmd is used to fire the first streaming call on program start.
func startStreamCmd(client ai.AIClient, prompt string) tea.Cmd {
	return func() tea.Msg {
		if sc, ok := client.(ai.StreamingAIClient); ok {
			deltaCh := make(chan string, 64)
			doneCh := make(chan error, 1)
			go func() {
				_, err := sc.StreamCommitMessage(context.Background(), prompt, func(d string) { deltaCh <- d })
				close(deltaCh)
				doneCh <- err
				close(doneCh)
			}()
			return streamStartedMsg{deltaCh: deltaCh, doneCh: doneCh}
		}
		// fallback
		msg, err := regenerate(prompt, client, "", "", false)
		return regenMsg{msg: msg, err: err}
	}
}

// readDeltaCmd reads a single delta from the channel (if available).
func readDeltaCmd(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		d, ok := <-ch
		if !ok {
			return nil
		}
		return streamDeltaMsg{delta: d}
	}
}

// waitDoneCmd waits for the completion error from the stream.
func waitDoneCmd(done <-chan error) tea.Cmd {
	return func() tea.Msg {
		err, ok := <-done
		if !ok {
			return streamDoneMsg{err: nil}
		}
		return streamDoneMsg{err: err}
	}
}

// regenerate performs a non-streaming AI call and normalizes the result.
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

func viewDiffCmd(_ string) tea.Cmd {
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

// --- helpers -----------------------------------------------------------------

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
