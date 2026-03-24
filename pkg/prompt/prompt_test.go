package prompt

import (
	"strings"
	"testing"
	"time"

	gogitobj "github.com/go-git/go-git/v5/plumbing/object"
	"github.com/renatogalera/ai-commit/pkg/committypes"
	"github.com/renatogalera/ai-commit/pkg/config"
)

func init() {
	committypes.InitCommitTypes([]config.CommitTypeConfig{
		{Type: "feat", Emoji: "✨"},
		{Type: "fix", Emoji: "🐛"},
		{Type: "docs", Emoji: "📚"},
		{Type: "refactor", Emoji: "♻️"},
		{Type: "chore", Emoji: "🔧"},
	})
}

func TestBuildCommitPrompt_DefaultTemplate(t *testing.T) {
	t.Parallel()
	result := BuildCommitPrompt("diff content", "English", "", "", "")

	if !strings.Contains(result, "diff content") {
		t.Error("expected prompt to contain diff")
	}
	if !strings.Contains(result, "English") {
		t.Error("expected prompt to contain language")
	}
	if !strings.Contains(result, "Conventional Commits") {
		t.Error("expected default template markers")
	}
}

func TestBuildCommitPrompt_CustomTemplate(t *testing.T) {
	t.Parallel()
	tmpl := "Generate commit for {DIFF} in {LANGUAGE}. {COMMIT_TYPE_HINT}{ADDITIONAL_CONTEXT}"
	result := BuildCommitPrompt("my diff", "Portuguese", "", "", tmpl)

	if !strings.Contains(result, "my diff") {
		t.Error("expected custom template to substitute diff")
	}
	if !strings.Contains(result, "Portuguese") {
		t.Error("expected custom template to substitute language")
	}
}

func TestBuildCommitPrompt_CommitTypeHint(t *testing.T) {
	t.Parallel()
	result := BuildCommitPrompt("diff", "English", "feat", "", "")

	if !strings.Contains(result, "feat") {
		t.Error("expected commit type hint for valid type")
	}

	// Invalid type should produce no hint
	result2 := BuildCommitPrompt("diff", "English", "invalidtype", "", "")
	if strings.Contains(result2, "Use the commit type 'invalidtype'") {
		t.Error("expected no hint for invalid commit type")
	}
}

func TestBuildCommitPrompt_AdditionalContext(t *testing.T) {
	t.Parallel()
	result := BuildCommitPrompt("diff", "English", "", "extra context here", "")

	if !strings.Contains(result, "Additional context provided by user") {
		t.Error("expected additional context header")
	}
	if !strings.Contains(result, "extra context here") {
		t.Error("expected additional context text")
	}

	// Empty additional text should not add context section
	result2 := BuildCommitPrompt("diff", "English", "", "", "")
	if strings.Contains(result2, "Additional context provided by user") {
		t.Error("expected no additional context when empty")
	}
}

func TestBuildCodeReviewPrompt_Default(t *testing.T) {
	t.Parallel()
	result := BuildCodeReviewPrompt("review diff", "English", "")

	if !strings.Contains(result, "review diff") {
		t.Error("expected diff in code review prompt")
	}
	if !strings.Contains(result, "English") {
		t.Error("expected language in code review prompt")
	}
	if !strings.Contains(result, "Review the following code diff") {
		t.Error("expected default code review template text")
	}
}

func TestBuildCodeReviewPrompt_Custom(t *testing.T) {
	t.Parallel()
	tmpl := "Review this: {DIFF} in {LANGUAGE}"
	result := BuildCodeReviewPrompt("my diff", "French", tmpl)

	if result != "Review this: my diff in French" {
		t.Errorf("got %q, expected custom template with substitutions", result)
	}
}

func TestBuildCommitStyleReviewPrompt_Default(t *testing.T) {
	t.Parallel()
	result := BuildCommitStyleReviewPrompt("feat: add login", "English", "")

	if !strings.Contains(result, "feat: add login") {
		t.Error("expected commit message in style review")
	}
	if !strings.Contains(result, "English") {
		t.Error("expected language in style review")
	}
}

func TestBuildCommitStyleReviewPrompt_Custom(t *testing.T) {
	t.Parallel()
	tmpl := "Check style: {COMMIT_MESSAGE} ({LANGUAGE})"
	result := BuildCommitStyleReviewPrompt("fix: bug", "Spanish", tmpl)

	if result != "Check style: fix: bug (Spanish)" {
		t.Errorf("got %q", result)
	}
}

func TestBuildCommitSummaryPrompt(t *testing.T) {
	t.Parallel()
	commit := &gogitobj.Commit{
		Author: gogitobj.Signature{
			Name:  "Test Author",
			Email: "test@example.com",
			When:  time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		Message: "feat: add new feature",
	}

	result := BuildCommitSummaryPrompt(commit, "some diff", "", "English")

	if !strings.Contains(result, "Test Author") {
		t.Error("expected author name")
	}
	if !strings.Contains(result, "feat: add new feature") {
		t.Error("expected commit message")
	}
	if !strings.Contains(result, "some diff") {
		t.Error("expected diff")
	}
	if !strings.Contains(result, "English") {
		t.Error("expected language")
	}
}

func TestBuildCommitSummaryPrompt_CustomTemplate(t *testing.T) {
	t.Parallel()
	commit := &gogitobj.Commit{
		Author: gogitobj.Signature{
			Name: "Dev",
			When: time.Now(),
		},
		Message: "test commit",
	}
	tmpl := "Author: {AUTHOR}, Diff: {DIFF}, Lang: {LANGUAGE}"
	result := BuildCommitSummaryPrompt(commit, "the diff", tmpl, "German")

	if !strings.Contains(result, "Author: Dev") {
		t.Error("expected author substitution")
	}
	if !strings.Contains(result, "Diff: the diff") {
		t.Error("expected diff substitution")
	}
	if !strings.Contains(result, "Lang: German") {
		t.Error("expected language substitution")
	}
}

func TestExtractSummaryAfterGeneral(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "with ### General Summary marker",
			input: "Some preamble\n### General Summary\n- Main changes",
			want:  "### General Summary\n- Main changes",
		},
		{
			name:  "with General Summary (no ###)",
			input: "Intro text\nGeneral Summary\n- Details",
			want:  "General Summary\n- Details",
		},
		{
			name:  "no marker returns full input",
			input: "No summary markers here",
			want:  "No summary markers here",
		},
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
		{
			name:  "marker at start",
			input: "### General Summary\n- Everything",
			want:  "### General Summary\n- Everything",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExtractSummaryAfterGeneral(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
