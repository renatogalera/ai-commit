package splitter

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/renatogalera/ai-commit/pkg/ai"
	"github.com/renatogalera/ai-commit/pkg/git"
)

type splitterState int

const (
	stateList splitterState = iota
	stateSpinner
	stateCommitted
)

var (
	selectedChunkStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("212")) // Highlight color for selected chunks

	unselectedChunkStyle = lipgloss.NewStyle() // Default style for unselected chunks
)

// Model for interactive splitting.
type Model struct {
	state         splitterState
	chunks        []git.DiffChunk
	selected      map[int]bool
	aiClient      ai.AIClient
	commitResult  string
	totalChunks   int // Total chunks count for status
	selectedCount int // Count of selected chunks for status
	
	// Terminal dimensions
	width  int
	height int
}

// NewSplitterModel creates a new splitter model.
func NewSplitterModel(chunks []git.DiffChunk, client ai.AIClient) Model {
	return Model{
		state:         stateList,
		chunks:        chunks,
		selected:      make(map[int]bool),
		aiClient:      client,
		commitResult:  "",
		totalChunks:   len(chunks), // Initialize total chunks
		selectedCount: 0,           // Initialize selected count to 0
	}
}

// NewProgram creates a new Bubble Tea program for splitting.
func NewProgram(m Model) *tea.Program {
	return tea.NewProgram(m, tea.WithAltScreen())
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
		
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case " ":
			// Toggle selection for all chunks.
			for i := range m.chunks {
				m.selected[i] = !m.selected[i]
			}
			m.updateSelectedCount() // Update selected count
		case "c":
			return m.updateCommit()
		case "a":
			for i := range m.chunks {
				m.selected[i] = true
			}
			m.updateSelectedCount() // Update count
		case "i":
			for i := range m.chunks {
				m.selected[i] = !m.selected[i]
			}
			m.updateSelectedCount() // Update count
		}
	}
	return m, nil
}

func (m Model) View() string {
	switch m.state {
	case stateList:
		return m.listView()
	case stateSpinner:
		return "Committing selected chunks..."
	case stateCommitted:
		return m.commitResult + "\nPress 'q' to exit."
	}
	return ""
}

func (m Model) listView() string {
	var b strings.Builder
	b.WriteString("Select chunks to commit (space to toggle, 'c' to commit, 'a' to select all, 'i' to invert selection, 'q' to quit):\n\n")
	for i, chunk := range m.chunks {
		marker := " "
		style := unselectedChunkStyle // Default unselected style
		if m.selected[i] {
			marker = "x"
			style = selectedChunkStyle // Apply selected style if chunk is selected
		}
		b.WriteString(fmt.Sprintf("[%s] %s\n", marker, style.Render(chunk.FilePath))) // Apply style to file path
	}
	footer := fmt.Sprintf("\nSelected chunks: %d/%d", m.selectedCount, m.totalChunks) // Show status footer
	b.WriteString(footer)

	return b.String()
}

func (m Model) updateCommit() (tea.Model, tea.Cmd) {
	m.state = stateSpinner
	return m, func() tea.Msg {
		err := partialCommit(m.chunks, m.selected, m.aiClient)
		if err != nil {
			m.commitResult = fmt.Sprintf("Error: %v", err)
		} else {
			m.commitResult = "Selected chunks committed successfully!"
		}
		m.state = stateCommitted
		return nil
	}
}

// updateSelectedCount recalculates and updates the count of selected chunks in the model.
func (m *Model) updateSelectedCount() {
	count := 0
	for _, isSelected := range m.selected {
		if isSelected {
			count++
		}
	}
	m.selectedCount = count
}

func partialCommit(chunks []git.DiffChunk, selected map[int]bool, client ai.AIClient) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	patch, err := buildPatch(chunks, selected)
	if err != nil {
		return err
	}
	if strings.TrimSpace(patch) == "" {
		return fmt.Errorf("no chunks selected")
	}
	cmd := exec.CommandContext(ctx, "git", "apply", "--cached", "-")
	cmd.Stdin = strings.NewReader(patch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to apply patch: %w", err)
	}

	partialDiff, err := git.GetGitDiffIgnoringMoves(ctx)
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
	return sb.String(), nil
}

func generatePartialCommitMessage(ctx context.Context, diff string, client ai.AIClient) (string, error) {
	prompt := fmt.Sprintf(`Generate a commit message for the following partial diff.
The message must follow Conventional Commits style.
Output only the commit message.

Diff:
%s
`, diff)
	msg, err := client.GetCommitMessage(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("AI error: %w", err)
	}
	return strings.TrimSpace(msg), nil
}

func RunInteractiveSplit(ctx context.Context, client ai.AIClient) error {
	diff, err := git.GetGitDiffIgnoringMoves(ctx)
	if err != nil {
		return err
	}
	diff = git.FilterLockFiles(diff, []string{"go.mod", "go.sum"})
	if strings.TrimSpace(diff) == "" {
		fmt.Println("No changes to commit (after filtering lock files). Did you stage your changes?")
		return nil
	}
	chunks, err := git.ParseDiffToChunks(diff)
	if err != nil {
		return fmt.Errorf("parseDiffToChunks error: %w", err)
	}
	if len(chunks) == 0 {
		fmt.Println("No diff chunks found.")
		return nil
	}
	model := NewSplitterModel(chunks, client)
	prog := NewProgram(model)
	return prog.Start()
}
