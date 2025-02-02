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
	gogpt "github.com/sashabaranov/go-openai"

	"github.com/renatogalera/ai-commit/pkg/committypes"
	"github.com/renatogalera/ai-commit/pkg/git"
	"github.com/renatogalera/ai-commit/pkg/openai"
	"github.com/renatogalera/ai-commit/pkg/template"
)

// uiState represents the different states of the TUI.
type uiState int

const (
	stateShowCommit    uiState = iota // Shows generated commit
	stateGenerating                   // Shows spinner while generating a new commit
	stateCommitting                   // Shows spinner while committing
	stateResult                       // Shows final result or error
	stateSelectType                   // Allows selecting commit type
	stateEditing                      // Allows editing the commit message
	stateEditingPrompt                // Allows editing the custom prompt
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
	state     uiState
	commitMsg string
	result    string
	spinner   spinner.Model

	// We store the original diff/language so we can rebuild the prompt if user changes it
	diff       string
	language   string
	prompt     string
	commitType string
	template   string

	enableEmoji   bool
	openAIClient  *gogpt.Client
	selectedIndex int
	commitTypes   []string
	regenCount    int
	maxRegens     int

	// Text area for editing the commit message or custom prompt
	textarea textarea.Model
}

// Style for the "help" instructions
var helpStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("212")).
	Bold(true)

// NewUIModel creates a new UI model with the provided parameters.
func NewUIModel(
	commitMsg string,
	diff string,
	language string,
	prompt string,
	commitType string,
	tmpl string,
	enableEmoji bool,
	client *gogpt.Client,
) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot

	ta := textarea.New()
	ta.Placeholder = "Edit here..."
	ta.Prompt = "> "
	ta.CharLimit = 0 // No character limit
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
		openAIClient:  client,
		spinner:       s,
		selectedIndex: 0,
		commitTypes:   committypes.AllTypes(),
		regenCount:    0,
		maxRegens:     3, // Limit the user to 3 regenerations
		textarea:      ta,
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
		//----------------------------------------------------------------------
		// MAIN SCREEN: stateShowCommit
		//----------------------------------------------------------------------
		case stateShowCommit:
			switch msg.String() {
			case "y", "enter":
				// User confirms commit
				m.state = stateCommitting
				return m, commitCmd(m.commitMsg)

			case "r":
				// User wants to regenerate
				if m.regenCount >= m.maxRegens {
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
				// User wants to select commit type
				m.state = stateSelectType
				return m, nil

			case "e":
				// User wants to edit the commit message
				m.state = stateEditing
				// Fill textarea with current commit message
				m.textarea.SetValue(m.commitMsg)
				m.textarea.Focus()
				return m, nil

			case "p":
				// User wants to edit the custom prompt
				m.state = stateEditingPrompt
				m.textarea.SetValue("") // Start blank or you could show existing
				m.textarea.Focus()
				return m, nil
			}

		//----------------------------------------------------------------------
		// SELECT TYPE SCREEN: stateSelectType
		//----------------------------------------------------------------------
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
				// Apply commit type, then regenerate
				m.commitType = m.commitTypes[m.selectedIndex]
				m.state = stateGenerating
				m.spinner = spinner.New()
				m.spinner.Spinner = spinner.Dot
				m.regenCount++
				// Build a fresh prompt with the new commit type
				m.prompt = openai.BuildPrompt(m.diff, m.language, m.commitType, "")
				return m, regenCmd(m.openAIClient, m.prompt, m.commitType, m.template, m.enableEmoji)
			}

		//----------------------------------------------------------------------
		// EDITING COMMIT MESSAGE: stateEditing
		//----------------------------------------------------------------------
		case stateEditing:
			// Pass the keypress to the textarea
			var tcmd tea.Cmd
			m.textarea, tcmd = m.textarea.Update(msg)
			cmd = tea.Batch(cmd, tcmd)

			switch msg.String() {
			case "esc":
				// Discard changes and return
				m.state = stateShowCommit
				return m, cmd

			case "ctrl+s":
				// Save changes and return to main screen
				m.commitMsg = m.textarea.Value()
				m.state = stateShowCommit
				return m, cmd
			}

		//----------------------------------------------------------------------
		// EDITING CUSTOM PROMPT: stateEditingPrompt
		//----------------------------------------------------------------------
		case stateEditingPrompt:
			var tcmd tea.Cmd
			m.textarea, tcmd = m.textarea.Update(msg)
			cmd = tea.Batch(cmd, tcmd)

			switch msg.String() {
			case "esc":
				// Discard changes and return
				m.state = stateShowCommit
				return m, cmd

			case "ctrl+s":
				// The user has added custom prompt text;
				// we rebuild the prompt, then regenerate
				userPrompt := m.textarea.Value()
				m.state = stateGenerating
				m.spinner = spinner.New()
				m.spinner.Spinner = spinner.Dot
				m.regenCount++

				// Rebuild the prompt with the additional text
				m.prompt = openai.BuildPrompt(m.diff, m.language, m.commitType, userPrompt)
				return m, regenCmd(m.openAIClient, m.prompt, m.commitType, m.template, m.enableEmoji)
			}

		//----------------------------------------------------------------------
		// RESULT SCREEN: stateResult
		//----------------------------------------------------------------------
		case stateResult:
			return m, tea.Quit
		}

	//----------------------------------------------------------------------
	// SPINNER MESSAGES AND OTHERS
	//----------------------------------------------------------------------
	case regenMsg:
		// Result from regenerate
		if msg.err != nil {
			m.result = fmt.Sprintf("Error generating commit message: %v", msg.err)
			m.state = stateResult
		} else {
			m.commitMsg = msg.msg
			m.state = stateShowCommit
		}

	case commitResultMsg:
		// Result from commit command
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

	// We might have sub-updates that return commands, in case we had lists, etc.
	return m, cmd
}

// View renders the UI based on the current state.
func (m Model) View() string {
	switch m.state {
	case stateShowCommit:
		helpText := helpStyle.Render(
			"Press 'y' to commit, 'r' to regenerate,\n" +
				"'e' to edit message, 't' to change commit type,\n" +
				"'p' to add custom prompt, or 'q' to quit.",
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
			b.WriteString(fmt.Sprintf("%s %s\n", cursor, ct))
		}
		b.WriteString("\nUse up/down arrows (or j/k) to navigate, enter to select,\n'q' or esc to go back.\n")
		return b.String()

	case stateEditing:
		// Show the text area for editing the commit message
		return fmt.Sprintf(
			"Editing commit message (Press ESC to discard, Ctrl+S to save):\n\n%s",
			m.textarea.View(),
		)

	case stateEditingPrompt:
		return fmt.Sprintf(
			"Add custom prompt text (Press ESC to discard, Ctrl+S to apply):\n\n%s",
			m.textarea.View(),
		)
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
