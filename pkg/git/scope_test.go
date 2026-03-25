package git

import "testing"

func TestScopeFromPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		path string
		want string
	}{
		{"pkg git", "pkg/git/git.go", "git"},
		{"pkg prompt", "pkg/prompt/prompt.go", "prompt"},
		{"pkg config", "pkg/config/config.go", "config"},
		{"pkg provider openai", "pkg/provider/openai/openai.go", "openai"},
		{"pkg provider anthropic", "pkg/provider/anthropic/client.go", "anthropic"},
		{"cmd ai-commit", "cmd/ai-commit/ai-commit.go", "cli"},
		{"cmd other", "cmd/other/main.go", "cli"},
		{"internal testutil", "internal/testutil/mock.go", "testutil"},
		{"root file", "main.go", ""},
		{"root config", "go.mod", ""},
		{"scripts dir", "scripts/install.sh", "scripts"},
		{"docs dir", "docs/README.md", "docs"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := scopeFromPath(tt.path)
			if got != tt.want {
				t.Errorf("scopeFromPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestSuggestScope(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		diff string
		want string
	}{
		{
			name: "single pkg",
			diff: "diff --git a/pkg/git/git.go b/pkg/git/git.go\n+code\ndiff --git a/pkg/git/scope.go b/pkg/git/scope.go\n+more",
			want: "git",
		},
		{
			name: "single provider",
			diff: "diff --git a/pkg/provider/openai/openai.go b/pkg/provider/openai/openai.go\n+code\ndiff --git a/pkg/provider/openai/register.go b/pkg/provider/openai/register.go\n+more",
			want: "openai",
		},
		{
			name: "cmd only",
			diff: "diff --git a/cmd/ai-commit/ai-commit.go b/cmd/ai-commit/ai-commit.go\n+code",
			want: "cli",
		},
		{
			name: "two scopes tie picks shorter",
			diff: "diff --git a/cmd/ai-commit/ai-commit.go b/cmd/ai-commit/ai-commit.go\n+code\ndiff --git a/pkg/prompt/prompt.go b/pkg/prompt/prompt.go\n+code",
			want: "cli",
		},
		{
			name: "three way tie returns empty",
			diff: "diff --git a/pkg/git/git.go b/pkg/git/git.go\n+a\ndiff --git a/pkg/prompt/prompt.go b/pkg/prompt/prompt.go\n+b\ndiff --git a/pkg/config/config.go b/pkg/config/config.go\n+c",
			want: "",
		},
		{
			name: "dominant scope wins",
			diff: "diff --git a/pkg/git/git.go b/pkg/git/git.go\n+a\ndiff --git a/pkg/git/scope.go b/pkg/git/scope.go\n+b\ndiff --git a/pkg/config/config.go b/pkg/config/config.go\n+c",
			want: "git",
		},
		{
			name: "root files only",
			diff: "diff --git a/main.go b/main.go\n+code\ndiff --git a/README.md b/README.md\n+doc",
			want: "",
		},
		{
			name: "empty diff",
			diff: "",
			want: "",
		},
		{
			name: "no diff headers",
			diff: "+just some code\n-removed line",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := SuggestScope(tt.diff)
			if got != tt.want {
				t.Errorf("SuggestScope() = %q, want %q", got, tt.want)
			}
		})
	}
}
