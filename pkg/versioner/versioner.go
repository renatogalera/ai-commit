package versioner

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/mod/semver"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/renatogalera/ai-commit/pkg/ai"
)

// GetCurrentVersionTag retrieves the latest semantic version tag.
func GetCurrentVersionTag(ctx context.Context) (string, error) {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}
	tagIter, err := repo.Tags()
	if err != nil {
		return "", fmt.Errorf("failed to retrieve tags: %w", err)
	}
	var latestTag string
	err = tagIter.ForEach(func(ref *plumbing.Reference) error {
		tagName := ref.Name().Short()
		if strings.HasPrefix(tagName, "v") && semver.IsValid(tagName) {
			if latestTag == "" || semver.Compare(tagName, latestTag) > 0 {
				latestTag = tagName
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return latestTag, nil
}

// SuggestNextVersion uses AI to suggest the next semantic version.
func SuggestNextVersion(ctx context.Context, currentVersion, commitMsg string, client ai.AIClient) (string, error) {
	if currentVersion == "" {
		currentVersion = "v0.0.0"
	}
	prompt := buildVersionPrompt(currentVersion, commitMsg)
	aiResponse, err := client.GetCommitMessage(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to get version suggestion: %w", err)
	}
	suggested, err := parseAiVersionSuggestion(aiResponse, currentVersion)
	if err != nil {
		return "", fmt.Errorf("failed to parse version suggestion: %w", err)
	}
	return suggested, nil
}

// CreateLocalTag creates a new Git tag with the provided version.
func CreateLocalTag(ctx context.Context, newVersionTag string) error {
	if newVersionTag == "" {
		return errors.New("version tag is empty")
	}
	repo, err := git.PlainOpen(".")
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}
	headRef, err := repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD reference: %w", err)
	}
	_, err = repo.CreateTag(newVersionTag, headRef.Hash(), nil)
	if err != nil {
		return fmt.Errorf("failed to create tag %s: %w", newVersionTag, err)
	}
	return nil
}

func buildVersionPrompt(currentVersion, commitMsg string) string {
	return fmt.Sprintf(`
We use semantic versioning: MAJOR.MINOR.PATCH.
The current version is %s.
Latest commit message:
"%s"

Based on the commit message, determine if the next version should be:
- MAJOR: breaking changes,
- MINOR: new features,
- PATCH: bug fixes or minor improvements.

Provide the next version in format vX.Y.Z without extra explanation.
`, currentVersion, commitMsg)
}

func parseAiVersionSuggestion(aiResponse, fallback string) (string, error) {
	re := regexp.MustCompile(`v?(\d+\.\d+\.\d+)`)
	match := re.FindStringSubmatch(aiResponse)
	if len(match) < 2 {
		return incrementPatch(fallback), nil
	}
	suggestedVersion := "v" + match[1]
	return suggestedVersion, nil
}

func incrementPatch(versionTag string) string {
	ver := stripLeadingV(versionTag)
	parts := strings.Split(ver, ".")
	if len(parts) != 3 {
		return "v0.0.1"
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "v0.0.1"
	}
	parts[2] = strconv.Itoa(patch + 1)
	return "v" + strings.Join(parts, ".")
}

func stripLeadingV(version string) string {
	if strings.HasPrefix(version, "v") {
		return strings.TrimPrefix(version, "v")
	}
	return version
}

// Semantic version TUI model
type semverChoice struct {
	label  string
	detail string
}

type semverModel struct {
	choices       []semverChoice
	cursor        int
	selected      bool
	selectedValue string
	currentVer    string
	
	// Terminal dimensions
	width  int
	height int
}

func NewSemverModel(currentVersion string) semverModel {
	clean := stripLeadingV(currentVersion)
	if clean == "" || !semver.IsValid("v"+clean) {
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
	return tea.Batch(
		tea.EnterAltScreen,
	)
}

func (m semverModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
		
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
		return fmt.Sprintf("Selected version: %s. Press any key to exit.\n", m.selectedValue)
	}
	s := fmt.Sprintf("Current version: %s\n\nSelect the next version:\n\n", m.currentVer)
	for i, choice := range m.choices {
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		s += fmt.Sprintf("%s %s => %s\n", cursor, choice.label, choice.detail)
	}
	s += "\nUse up/down (or j/k) to navigate, enter to select, 'q' to cancel.\n"
	return s
}

func parseVersionTriplet(ver string) (int, int, int) {
	parts := strings.Split(ver, ".")
	if len(parts) != 3 {
		return 0, 0, 0
	}
	major, _ := strconv.Atoi(parts[0])
	minor, _ := strconv.Atoi(parts[1])
	patch, _ := strconv.Atoi(parts[2])
	return major, minor, patch
}

// RunSemVerTUI launches the semantic version TUI and returns the selected version.
func RunSemVerTUI(ctx context.Context, currentVersion string) (string, error) {
	model := NewSemverModel(currentVersion)
	program := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := program.Run()
	if err != nil {
		return "", err
	}
	m, ok := finalModel.(semverModel)
	if !ok || !m.selected {
		return "", nil
	}
	return m.selectedValue, nil
}

// PerformSemanticRelease performs the semantic version bump process.
func PerformSemanticRelease(ctx context.Context, client ai.AIClient, commitMsg string, manual bool) error {
	currentVersion, err := GetCurrentVersionTag(ctx)
	if err != nil {
		return fmt.Errorf("could not retrieve current version: %w", err)
	}
	if currentVersion == "" {
		currentVersion = "v0.0.0"
	}
	var nextVersion string
	if manual {
		nextVersion, err = RunSemVerTUI(ctx, currentVersion)
		if err != nil {
			return fmt.Errorf("manual semantic version selection failed: %w", err)
		}
		if nextVersion == "" {
			return nil
		}
	} else {
		nextVersion, err = SuggestNextVersion(ctx, currentVersion, commitMsg, client)
		if err != nil {
			return fmt.Errorf("AI version suggestion failed: %w", err)
		}
	}
	if err := CreateLocalTag(ctx, nextVersion); err != nil {
		return fmt.Errorf("failed to create tag %s: %w", nextVersion, err)
	}
	return nil
}
