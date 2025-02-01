package splitter

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/renatogalera/ai-commit/pkg/git"
	"github.com/renatogalera/ai-commit/pkg/openai"
)

// splitterState enumerates possible states in the TUI
type splitterState int

const (
	stateList splitterState = iota
	stateSpinner
	stateCommitted
)

// chunkItem is the list.Item implementation for Bubbles
type chunkItem struct {
	Chunk    git.DiffChunk
	Selected bool
}

func (ci chunkItem) Title() string       { return ci.Chunk.FilePath }
func (ci chunkItem) Description() string { return ci.Chunk.HunkHeader }
func (ci chunkItem) FilterValue() string { return ci.Chunk.FilePath }

// Model for interactive splitting
type Model struct {
	state        splitterState
	list         list.Model
	spinner      spinner.Model
	chunks       []git.DiffChunk
	selected     map[int]bool
	apiKey       string
	commitResult string
}

func NewSplitterModel(chunks []git.DiffChunk, apiKey string) Model {
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
		state:    stateList,
		list:     l,
		spinner:  s,
		chunks:   chunks,
		selected: make(map[int]bool),
		apiKey:   apiKey,
	}
}

func NewProgram(m Model) *tea.Program {
	return tea.NewProgram(m)
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-2)
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		// toggle chunk selection
		case " ":
			index := m.list.Index()
			m.selected[index] = !m.selected[index]
			return m, nil
		// commit selected chunks
		case "c":
			return m.updateCommit()
		// demonstrate an "auto-group" approach
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
	// Let the list handle its own updates
	newList, cmd := m.list.Update(msg)
	m.list = newList
	return m, cmd
}

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

// updateCommit handles partial staging, AI message generation, commit creation
func (m Model) updateCommit() (tea.Model, tea.Cmd) {
	// Mark state as spinner
	newModel := m
	newModel.state = stateSpinner

	// We'll do the partial commit in a command
	return newModel, func() tea.Msg {
		if err := partialCommit(m.chunks, m.selected, m.apiKey); err != nil {
			newModel.commitResult = fmt.Sprintf("Error: %v", err)
		} else {
			newModel.commitResult = "Selected chunks committed successfully!"
		}
		newModel.state = stateCommitted
		return nil
	}
}

// updateAutoGroup simulates chunk grouping by AI
func (m Model) updateAutoGroup() (tea.Model, tea.Cmd) {
	// For demonstration, let's say "select all" or something more advanced
	for i := range m.chunks {
		m.selected[i] = true
	}
	return m, nil
}

// partialCommit:
// 1) resets all staged changes
// 2) builds a patch from selected chunks
// 3) applies the patch to stage only those lines
// 4) uses AI to generate the commit message
// 5) commits them
func partialCommit(chunks []git.DiffChunk, selected map[int]bool, apiKey string) error {
	// 1) reset staged changes
	if err := run("git", "reset"); err != nil {
		return fmt.Errorf("failed to reset: %w", err)
	}

	// 2) build patch from selected chunks
	patch, err := buildPatch(chunks, selected)
	if err != nil {
		return err
	}
	if patch == "" {
		return fmt.Errorf("no chunks selected")
	}

	// 3) apply patch
	cmd := exec.Command("git", "apply", "--cached", "-")
	cmd.Stdin = strings.NewReader(patch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to apply patch: %w", err)
	}

	// 4) generate commit message
	diff, err := git.GetGitDiff()
	if err != nil {
		return fmt.Errorf("failed to get partial diff: %w", err)
	}
	prompt := buildCommitPrompt(diff)
	commitMsg, err := openai.GetChatCompletion(nil, prompt, apiKey)
	if err != nil {
		return fmt.Errorf("AI error: %w", err)
	}

	// 5) create commit
	if err := git.CommitChanges(commitMsg); err != nil {
		return err
	}
	return nil
}

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
			// line might be +, -, or context
			sb.WriteString(line + "\n")
		}
	}
	patch := sb.String()
	if strings.TrimSpace(patch) == "" {
		return "", nil
	}
	return patch, nil
}

// buildCommitPrompt is a simplified prompt for AI
func buildCommitPrompt(diff string) string {
	return fmt.Sprintf(`
Generate a commit message for the following partial diff.
The commit message must follow the Conventional Commits style.
Output ONLY the commit message.

Diff:
%s
`, diff)
}

func run(cmdName string, args ...string) error {
	cmd := exec.Command(cmdName, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
