package versioner

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/mod/semver"
)

// semverChoice represents one line in our TUI – major/minor/patch, plus the resulting version string.
type semverChoice struct {
	label  string
	detail string
}

// semverModel is our Bubble Tea model for picking a new semver.
type semverModel struct {
	choices       []semverChoice
	cursor        int
	selected      bool
	selectedValue string
	currentVer    string
}

func NewSemverModel(currentVersion string) semverModel {
	clean := stripLeadingV(currentVersion)
	if !strings.HasPrefix(clean, "0.0.0") && !semver.IsValid("v"+clean) {
		clean = "0.0.0"
	}

	major, minor, patch := parseVersionTriplet(clean)
	majorChoice := fmt.Sprintf("v%d.0.0", major+1)
	minorChoice := fmt.Sprintf("v%d.%d.0", major, minor+1)
	patchChoice := fmt.Sprintf("v%d.%d.%d", major, minor, patch+1)

	return semverModel{
		choices: []semverChoice{
			{label: "Major", detail: majorChoice},
			{label: "Minor", detail: minorChoice},
			{label: "Patch", detail: patchChoice},
		},
		cursor:     0,
		currentVer: currentVersion,
	}
}

func (m semverModel) Init() tea.Cmd {
	return nil
}

func (m semverModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter":
			m.selected = true
			m.selectedValue = m.choices[m.cursor].detail
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m semverModel) View() string {
	if m.selected {
		return fmt.Sprintf("You selected %s. Press any key to exit.\n", m.selectedValue)
	}

	s := fmt.Sprintf("Current version: %s\n\n", m.currentVer)
	s += "Select the next version (↑/↓, enter, or q to quit):\n\n"
	for i, choice := range m.choices {
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		s += fmt.Sprintf("%s %s => %s\n", cursor, choice.label, choice.detail)
	}
	return s + "\n"
}

// RunSemVerTUI runs the TUI program and returns the chosen version or empty if canceled.
func RunSemVerTUI(ctx context.Context, currentVersion string) (string, error) {
	initialModel := NewSemverModel(currentVersion)
	p := tea.NewProgram(initialModel)
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}

	m, ok := finalModel.(semverModel)
	if !ok {
		return "", fmt.Errorf("unexpected model type")
	}
	if !m.selected {
		return "", nil
	}
	return m.selectedValue, nil
}

func parseVersionTriplet(ver string) (int, int, int) {
	parts := strings.Split(ver, ".")
	if len(parts) < 3 {
		return 0, 0, 0
	}
	var major, minor, patch int
	fmt.Sscan(parts[0], &major)
	fmt.Sscan(parts[1], &minor)
	fmt.Sscan(parts[2], &patch)
	return major, minor, patch
}
