package committypes

import (
	"strings"
	"testing"

	"github.com/renatogalera/ai-commit/pkg/config"
)

func setupTypes(t *testing.T) {
	t.Helper()
	InitCommitTypes([]config.CommitTypeConfig{
		{Type: "feat", Emoji: "✨"},
		{Type: "fix", Emoji: "🐛"},
		{Type: "docs", Emoji: "📚"},
		{Type: "style", Emoji: "💎"},
		{Type: "refactor", Emoji: "♻️"},
		{Type: "test", Emoji: "🧪"},
		{Type: "chore", Emoji: "🔧"},
		{Type: "perf", Emoji: "🚀"},
		{Type: "build", Emoji: "📦"},
		{Type: "ci", Emoji: "👷"},
	})
}

func TestInitCommitTypes(t *testing.T) {
	setupTypes(t)
	types := GetAllTypes()
	if len(types) != 10 {
		t.Errorf("expected 10 types, got %d", len(types))
	}

	// Re-init should replace, not append
	InitCommitTypes([]config.CommitTypeConfig{
		{Type: "feat", Emoji: "✨"},
	})
	types = GetAllTypes()
	if len(types) != 1 {
		t.Errorf("expected 1 type after reinit, got %d", len(types))
	}

	// Restore for other tests
	setupTypes(t)
}

func TestInitCommitTypes_TrimsWhitespace(t *testing.T) {
	InitCommitTypes([]config.CommitTypeConfig{
		{Type: "  feat  ", Emoji: "  ✨  "},
	})
	if !IsValidCommitType("feat") {
		t.Error("expected trimmed type to be valid")
	}
	if GetEmojiForType("feat") != "✨" {
		t.Error("expected trimmed emoji")
	}
	setupTypes(t) // restore
}

func TestIsValidCommitType(t *testing.T) {
	setupTypes(t)
	tests := []struct {
		name     string
		typ      string
		expected bool
	}{
		{"valid feat", "feat", true},
		{"valid fix", "fix", true},
		{"valid docs", "docs", true},
		{"valid refactor", "refactor", true},
		{"valid chore", "chore", true},
		{"valid perf", "perf", true},
		{"valid build", "build", true},
		{"valid ci", "ci", true},
		{"invalid type", "invalid", false},
		{"empty string", "", false},
		{"case sensitive", "Feat", false},
		{"partial match", "fea", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidCommitType(tt.typ)
			if got != tt.expected {
				t.Errorf("IsValidCommitType(%q) = %v, want %v", tt.typ, got, tt.expected)
			}
		})
	}
}

func TestGetEmojiForType(t *testing.T) {
	setupTypes(t)
	tests := []struct {
		name string
		typ  string
		want string
	}{
		{"feat emoji", "feat", "✨"},
		{"fix emoji", "fix", "🐛"},
		{"docs emoji", "docs", "📚"},
		{"unknown returns empty", "unknown", ""},
		{"empty returns empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetEmojiForType(tt.typ)
			if got != tt.want {
				t.Errorf("GetEmojiForType(%q) = %q, want %q", tt.typ, got, tt.want)
			}
		})
	}
}

func TestGuessCommitType(t *testing.T) {
	setupTypes(t)
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{"detects feat", "feat: add new login", "feat"},
		{"detects feat in sentence", "add a new feat to the system", "feat"},
		{"detects fix", "fix null pointer exception", "fix"},
		{"detects docs", "update docs for API", "docs"},
		{"detects refactor", "refactor the auth module", "refactor"},
		{"detects chore", "chore: update dependencies", "chore"},
		{"detects test", "add unit test for parser", "test"},
		{"detects perf", "improve perf for query", "perf"},
		{"detects build", "update build configuration", "build"},
		{"detects ci", "update ci configuration", "ci"},
		{"detects style", "fix style issues", "style"},
		{"no match returns empty", "do something random", ""},
		{"empty message", "", ""},
		{"word boundary: feature does not match feat", "add new feature", ""},
		{"word boundary: prefix does not match fix", "the prefix was wrong", ""},
		{"longer type wins: refactor over fix", "refactor to fix issue", "refactor"},
		{"case insensitive fix", "fix the broken code", "fix"},
		{"multiline uses first line", "feat something\nfix something else", "feat"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GuessCommitType(tt.message)
			if got != tt.want {
				t.Errorf("GuessCommitType(%q) = %q, want %q", tt.message, got, tt.want)
			}
		})
	}
}

func TestTypesRegexPattern(t *testing.T) {
	setupTypes(t)
	pattern := TypesRegexPattern()

	// Should contain all types
	for _, typ := range []string{"feat", "fix", "docs", "refactor", "chore", "test", "perf", "build", "ci", "style"} {
		if !strings.Contains(pattern, typ) {
			t.Errorf("pattern should contain %q, got %q", typ, pattern)
		}
	}
	// Should be pipe-separated
	if !strings.Contains(pattern, "|") {
		t.Error("pattern should be pipe-separated")
	}
}

func TestTypesRegexPattern_EmptyList(t *testing.T) {
	InitCommitTypes(nil)
	// Force empty list
	commitTypeList = nil
	pattern := TypesRegexPattern()
	if pattern == "" {
		t.Error("expected fallback pattern when list is empty")
	}
	if !strings.Contains(pattern, "feat") {
		t.Error("expected fallback to contain feat")
	}
	setupTypes(t) // restore
}

func TestBuildRegexPatternWithEmoji(t *testing.T) {
	setupTypes(t)
	re := BuildRegexPatternWithEmoji()

	tests := []struct {
		name  string
		input string
		match bool
	}{
		{"simple type prefix", "feat: add login", true},
		{"type with scope", "fix(auth): resolve bug", true},
		{"emoji prefix", "✨ feat: add feature", true},
		{"no type prefix", "add something", false},
		{"invalid type", "invalid: something", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := re.MatchString(tt.input)
			if got != tt.match {
				t.Errorf("regex.MatchString(%q) = %v, want %v", tt.input, got, tt.match)
			}
		})
	}
}

func TestGetAllTypes(t *testing.T) {
	setupTypes(t)
	types := GetAllTypes()
	if len(types) != 10 {
		t.Errorf("expected 10 types, got %d", len(types))
	}
	// Verify order matches init order
	if types[0] != "feat" {
		t.Errorf("expected first type to be feat, got %q", types[0])
	}
}
