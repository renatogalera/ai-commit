package splitter

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	gogpt "github.com/sashabaranov/go-openai"

	"github.com/renatogalera/ai-commit/pkg/git"
	"github.com/renatogalera/ai-commit/pkg/openai"
)

// splitterState represents the state of the interactive splitter UI.
type splitterState int

const (
	stateList splitterState = iota
	stateSpinner
	stateCommitted
)

// chunkItem implements list.Item for displaying diff chunks.
type chunkItem struct {
	Chunk    git.DiffChunk
	Selected bool
}

func (ci chunkItem) Title() string       { return ci.Chunk.FilePath }
func (ci chunkItem) Description() string { return ci.Chunk.HunkHeader }
func (ci chunkItem) FilterValue() string { return ci.Chunk.FilePath }

// Model represents the state of the interactive splitter.
type Model struct {
	state        splitterState
	list         list.Model
	spinner      spinner.Model
	chunks       []git.DiffChunk
	selected     map[int]bool
	openAIClient *gogpt.Client
	commitResult string
}

// NewSplitterModel creates a new splitter model with the given diff chunks and OpenAI client.
func NewSplitterModel(chunks []git.DiffChunk, client *gogpt.Client) Model {
	items := make([]list.Item, 0, len(chunks))
	for _, c := range chunks {
		items = append(items, chunkItem{Chunk: c})
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select chunks to commit (press space to toggle, 'c' to commit, 'a' to auto-group, 'q' to quit)"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)

	s := spinner.New()
	s.Spinner = spinner.Dot

	return Model{
		state:        stateList,
		list:         l,
		spinner:      s,
		chunks:       chunks,
		selected:     make(map[int]bool),
		openAIClient: client,
	}
}

// NewProgram creates a new Bubble Tea program for the splitter UI.
func NewProgram(m Model) *tea.Program {
	return tea.NewProgram(m)
}

// Init is the initialization function for the splitter.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update updates the splitter model based on incoming messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-2)
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case " ":
			index := m.list.Index()
			m.selected[index] = !m.selected[index]
			return m, nil
		case "c":
			return m.updateCommit()
		case "a":
			return m.updateAutoGroup()
		}
	case spinner.TickMsg:
		if m.state == stateSpinner {
			newSpinner, cmd := m.spinner.Update(msg)
			m.spinner = newSpinner
			return m, cmd
		}
	}
	newList, cmd := m.list.Update(msg)
	m.list = newList
	return m, cmd
}

// View renders the splitter UI based on the current state.
func (m Model) View() string {
	switch m.state {
	case stateList:
		return m.list.View()
	case stateSpinner:
		return fmt.Sprintf("Working... %s", m.spinner.View())
	case stateCommitted:
		return m.commitResult + "\nPress q to exit."
	}
	return ""
}

// updateCommit handles the process of committing selected chunks.
func (m Model) updateCommit() (tea.Model, tea.Cmd) {
	newModel := m
	newModel.state = stateSpinner

	return newModel, func() tea.Msg {
		err := partialCommit(newModel.chunks, newModel.selected, newModel.openAIClient)
		if err != nil {
			newModel.commitResult = fmt.Sprintf("Error: %v", err)
		} else {
			newModel.commitResult = "Selected chunks committed successfully!"
		}
		newModel.state = stateCommitted
		return nil
	}
}

// updateAutoGroup selects all chunks for auto grouping.
func (m Model) updateAutoGroup() (tea.Model, tea.Cmd) {
	for i := range m.chunks {
		m.selected[i] = true
	}
	return m, nil
}

// partialCommit stages selected diff chunks and commits them with an AI-generated commit message.
func partialCommit(chunks []git.DiffChunk, selected map[int]bool, client *gogpt.Client) error {
	// Unstage everything first to apply patch exactly as selected.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := runCmd(ctx, "git", "reset"); err != nil {
		return fmt.Errorf("failed to reset: %w", err)
	}
	patch, err := buildPatch(chunks, selected)
	if err != nil {
		return err
	}
	if patch == "" {
		return fmt.Errorf("no chunks selected")
	}

	cmd := exec.CommandContext(ctx, "git", "apply", "--cached", "-")
	cmd.Stdin = strings.NewReader(patch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to apply patch: %w", err)
	}

	partialDiff, err := git.GetGitDiff(ctx)
	if err != nil {
		return fmt.Errorf("failed to get partial diff: %w", err)
	}
	commitMsg, err := generatePartialCommitMessage(ctx, partialDiff, client)
	if err != nil {
		return err
	}
	if err := git.CommitChanges(ctx, commitMsg); err != nil {
		return err
	}
	return nil
}

// buildPatch constructs a patch string from the selected diff chunks.
func buildPatch(chunks []git.DiffChunk, selected map[int]bool) (string, error) {
	var sb strings.Builder
	for i, c := range chunks {
		if !selected[i] {
			continue
		}
		sb.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", c.FilePath, c.FilePath))
		sb.WriteString("--- a/" + c.FilePath + "\n")
		sb.WriteString("+++ b/" + c.FilePath + "\n")
		sb.WriteString(c.HunkHeader + "\n")
		for _, line := range c.Lines {
			sb.WriteString(line + "\n")
		}
	}
	patch := sb.String()
	if strings.TrimSpace(patch) == "" {
		return "", nil
	}
	return patch, nil
}

// generatePartialCommitMessage uses the user’s partial diff to produce a commit message via OpenAI.
func generatePartialCommitMessage(ctx context.Context, diff string, client *gogpt.Client) (string, error) {
	prompt := fmt.Sprintf(`
Generate a commit message for the following partial diff.
The commit message must follow the Conventional Commits style.
Output ONLY the commit message.

Diff:
%s
`, diff)

	msg, err := openai.GetChatCompletion(ctx, client, prompt)
	if err != nil {
		return "", fmt.Errorf("AI error: %w", err)
	}
	return strings.TrimSpace(msg), nil
}

// runCmd executes a command with a context.
func runCmd(ctx context.Context, cmdName string, args ...string) error {
	cmd := exec.CommandContext(ctx, cmdName, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
