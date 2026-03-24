package versioner

import (
	"context"
	"fmt"
	"testing"
)

func TestIncrementPatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"standard version", "v1.2.3", "v1.2.4"},
		{"zero patch", "v1.0.0", "v1.0.1"},
		{"without v prefix", "1.2.3", "v1.2.4"},
		{"high numbers", "v10.20.99", "v10.20.100"},
		{"invalid format", "invalid", "v0.0.1"},
		{"empty string", "", "v0.0.1"},
		{"two parts only", "v1.2", "v0.0.1"},
		{"non-numeric patch", "v1.2.abc", "v0.0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := incrementPatch(tt.version)
			if got != tt.want {
				t.Errorf("incrementPatch(%q) = %q, want %q", tt.version, got, tt.want)
			}
		})
	}
}

func TestStripLeadingV(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"v1.2.3", "1.2.3"},
		{"1.2.3", "1.2.3"},
		{"v0.0.0", "0.0.0"},
		{"", ""},
		{"vv1.0.0", "v1.0.0"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := stripLeadingV(tt.input)
			if got != tt.want {
				t.Errorf("stripLeadingV(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseVersionTriplet(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ver                    string
		wantMajor, wantMinor, wantPatch int
	}{
		{"1.2.3", 1, 2, 3},
		{"0.0.0", 0, 0, 0},
		{"10.20.30", 10, 20, 30},
		{"invalid", 0, 0, 0},
		{"1.2", 0, 0, 0},
		{"", 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.ver, func(t *testing.T) {
			t.Parallel()
			major, minor, patch := parseVersionTriplet(tt.ver)
			if major != tt.wantMajor || minor != tt.wantMinor || patch != tt.wantPatch {
				t.Errorf("parseVersionTriplet(%q) = (%d,%d,%d), want (%d,%d,%d)",
					tt.ver, major, minor, patch, tt.wantMajor, tt.wantMinor, tt.wantPatch)
			}
		})
	}
}

func TestParseAiVersionSuggestion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		response string
		fallback string
		want     string
	}{
		{
			name:     "clean version",
			response: "v1.3.0",
			fallback: "v1.2.0",
			want:     "v1.3.0",
		},
		{
			name:     "version without v",
			response: "1.3.0",
			fallback: "v1.2.0",
			want:     "v1.3.0",
		},
		{
			name:     "version in sentence",
			response: "The next version should be v2.0.0 based on breaking changes.",
			fallback: "v1.9.0",
			want:     "v2.0.0",
		},
		{
			name:     "no version found falls back to patch increment",
			response: "I'm not sure what version to suggest",
			fallback: "v1.2.3",
			want:     "v1.2.4",
		},
		{
			name:     "empty response falls back",
			response: "",
			fallback: "v0.1.0",
			want:     "v0.1.1",
		},
		{
			name:     "multiple versions picks first",
			response: "v1.0.0 or v2.0.0",
			fallback: "v0.9.0",
			want:     "v1.0.0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseAiVersionSuggestion(tt.response, tt.fallback)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("parseAiVersionSuggestion(%q, %q) = %q, want %q",
					tt.response, tt.fallback, got, tt.want)
			}
		})
	}
}

func TestBuildVersionPrompt(t *testing.T) {
	t.Parallel()
	prompt := buildVersionPrompt("v1.0.0", "feat: add new feature")

	if !containsStr(prompt, "v1.0.0") {
		t.Error("expected current version in prompt")
	}
	if !containsStr(prompt, "feat: add new feature") {
		t.Error("expected commit message in prompt")
	}
	if !containsStr(prompt, "MAJOR") || !containsStr(prompt, "MINOR") || !containsStr(prompt, "PATCH") {
		t.Error("expected semver categories in prompt")
	}
}

func TestNewSemverModel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		version    string
		wantMajor  string
		wantMinor  string
		wantPatch  string
	}{
		{
			name:      "standard version",
			version:   "v1.2.3",
			wantMajor: "v2.0.0",
			wantMinor: "v1.3.0",
			wantPatch: "v1.2.4",
		},
		{
			name:      "zero version",
			version:   "v0.0.0",
			wantMajor: "v1.0.0",
			wantMinor: "v0.1.0",
			wantPatch: "v0.0.1",
		},
		{
			name:      "empty version defaults",
			version:   "",
			wantMajor: "v1.0.0",
			wantMinor: "v0.1.0",
			wantPatch: "v0.0.1",
		},
		{
			name:      "invalid version defaults",
			version:   "invalid",
			wantMajor: "v1.0.0",
			wantMinor: "v0.1.0",
			wantPatch: "v0.0.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := NewSemverModel(tt.version)
			if len(m.choices) != 3 {
				t.Fatalf("expected 3 choices, got %d", len(m.choices))
			}
			if m.choices[0].detail != tt.wantMajor {
				t.Errorf("Major = %q, want %q", m.choices[0].detail, tt.wantMajor)
			}
			if m.choices[1].detail != tt.wantMinor {
				t.Errorf("Minor = %q, want %q", m.choices[1].detail, tt.wantMinor)
			}
			if m.choices[2].detail != tt.wantPatch {
				t.Errorf("Patch = %q, want %q", m.choices[2].detail, tt.wantPatch)
			}
		})
	}
}

func TestSuggestNextVersion(t *testing.T) {
	t.Parallel()

	mockClient := &mockAIClient{
		response: "v1.1.0",
	}

	got, err := SuggestNextVersion(context.Background(), "v1.0.0", "feat: add feature", mockClient)
	if err != nil {
		t.Fatal(err)
	}
	if got != "v1.1.0" {
		t.Errorf("got %q, want v1.1.0", got)
	}
}

func TestSuggestNextVersion_EmptyCurrentVersion(t *testing.T) {
	t.Parallel()

	mockClient := &mockAIClient{
		response: "v0.1.0",
	}

	got, err := SuggestNextVersion(context.Background(), "", "feat: initial feature", mockClient)
	if err != nil {
		t.Fatal(err)
	}
	if got != "v0.1.0" {
		t.Errorf("got %q, want v0.1.0", got)
	}
}

func TestSuggestNextVersion_AIError(t *testing.T) {
	t.Parallel()

	mockClient := &mockAIClient{
		err: fmt.Errorf("AI service unavailable"),
	}

	_, err := SuggestNextVersion(context.Background(), "v1.0.0", "fix: something", mockClient)
	if err == nil {
		t.Error("expected error when AI fails")
	}
}

// mockAIClient implements ai.AIClient for testing.
type mockAIClient struct {
	response string
	err      error
}

func (m *mockAIClient) GetCommitMessage(_ context.Context, _ string) (string, error) {
	return m.response, m.err
}

func (m *mockAIClient) SanitizeResponse(msg, _ string) string { return msg }
func (m *mockAIClient) ProviderName() string                  { return "mock" }
func (m *mockAIClient) MaybeSummarizeDiff(d string, _ int) (string, bool) {
	return d, false
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
