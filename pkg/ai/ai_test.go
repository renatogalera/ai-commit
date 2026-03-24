package ai

import (
	"testing"

	"github.com/renatogalera/ai-commit/pkg/committypes"
	"github.com/renatogalera/ai-commit/pkg/config"
)

func init() {
	// Ensure commit types are initialized for SanitizeResponse regex.
	committypes.InitCommitTypes([]config.CommitTypeConfig{
		{Type: "feat", Emoji: "✨"},
		{Type: "fix", Emoji: "🐛"},
		{Type: "docs", Emoji: "📚"},
		{Type: "refactor", Emoji: "♻️"},
		{Type: "chore", Emoji: "🔧"},
		{Type: "test", Emoji: "🧪"},
		{Type: "perf", Emoji: "🚀"},
		{Type: "style", Emoji: "💎"},
		{Type: "build", Emoji: "📦"},
		{Type: "ci", Emoji: "👷"},
	})
}

func TestProviderName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		provider string
		want     string
	}{
		{"returns openai", "openai", "openai"},
		{"returns anthropic", "anthropic", "anthropic"},
		{"returns empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b := &BaseAIClient{Provider: tt.provider}
			if got := b.ProviderName(); got != tt.want {
				t.Errorf("ProviderName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSanitizeResponse(t *testing.T) {
	t.Parallel()
	b := &BaseAIClient{Provider: "test"}

	tests := []struct {
		name       string
		message    string
		commitType string
		want       string
	}{
		{
			name:       "removes backticks",
			message:    "```feat: add login```",
			commitType: "",
			want:       "feat: add login",
		},
		{
			name:       "trims whitespace",
			message:    "   add login feature   ",
			commitType: "",
			want:       "add login feature",
		},
		{
			name:       "strips type prefix when commitType given",
			message:    "feat: add login feature",
			commitType: "feat",
			want:       "add login feature",
		},
		{
			name:       "strips type with scope when commitType given",
			message:    "feat(auth): add login feature",
			commitType: "feat",
			want:       "add login feature",
		},
		{
			name:       "strips emoji and type prefix",
			message:    "✨ feat: add login feature",
			commitType: "feat",
			want:       "add login feature",
		},
		{
			name:       "preserves body lines",
			message:    "feat: add login\n\nDetailed description here",
			commitType: "feat",
			want:       "add login\n\nDetailed description here",
		},
		{
			name:       "no stripping when no commitType",
			message:    "feat: add login",
			commitType: "",
			want:       "feat: add login",
		},
		{
			name:       "handles fix type",
			message:    "fix(api): resolve null pointer",
			commitType: "fix",
			want:       "resolve null pointer",
		},
		{
			name:       "handles refactor type",
			message:    "refactor: extract function",
			commitType: "refactor",
			want:       "extract function",
		},
		{
			name:       "empty message",
			message:    "",
			commitType: "feat",
			want:       "",
		},
		{
			name:       "backticks with type stripping",
			message:    "```fix: resolve race condition```",
			commitType: "fix",
			want:       "resolve race condition",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := b.SanitizeResponse(tt.message, tt.commitType)
			if got != tt.want {
				t.Errorf("SanitizeResponse(%q, %q) = %q, want %q",
					tt.message, tt.commitType, got, tt.want)
			}
		})
	}
}

func TestMaybeSummarizeDiff(t *testing.T) {
	t.Parallel()
	b := &BaseAIClient{Provider: "test"}

	tests := []struct {
		name          string
		diff          string
		maxLength     int
		wantTruncated bool
		wantSuffix    string
	}{
		{
			name:          "within limit returns unchanged",
			diff:          "short diff",
			maxLength:     100,
			wantTruncated: false,
		},
		{
			name:          "exact limit returns unchanged",
			diff:          "12345",
			maxLength:     5,
			wantTruncated: false,
		},
		{
			name:          "over limit truncates at newline",
			diff:          "line1\nline2\nline3\nline4",
			maxLength:     12,
			wantTruncated: true,
			wantSuffix:    "\n[... diff truncated for brevity ...]",
		},
		{
			name:          "over limit without newline truncates at maxLength",
			diff:          "abcdefghijklmnop",
			maxLength:     10,
			wantTruncated: true,
			wantSuffix:    "\n[... diff truncated for brevity ...]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, truncated := b.MaybeSummarizeDiff(tt.diff, tt.maxLength)
			if truncated != tt.wantTruncated {
				t.Errorf("truncated = %v, want %v", truncated, tt.wantTruncated)
			}
			if !truncated && got != tt.diff {
				t.Errorf("got = %q, want original diff %q", got, tt.diff)
			}
			if truncated {
				if len(got) == 0 {
					t.Error("truncated result should not be empty")
				}
				if tt.wantSuffix != "" {
					suffix := got[len(got)-len(tt.wantSuffix):]
					if suffix != tt.wantSuffix {
						t.Errorf("got suffix %q, want %q", suffix, tt.wantSuffix)
					}
				}
			}
		})
	}
}
