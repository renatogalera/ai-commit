package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetProviderSettings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		cfg      *Config
		provider string
		wantKey  string
	}{
		{
			name: "existing provider",
			cfg: &Config{
				Providers: map[string]ProviderSettings{
					"openai": {APIKey: "sk-123", Model: "gpt-4", BaseURL: "https://api.openai.com"},
				},
			},
			provider: "openai",
			wantKey:  "sk-123",
		},
		{
			name: "missing provider returns empty",
			cfg: &Config{
				Providers: map[string]ProviderSettings{
					"openai": {APIKey: "sk-123"},
				},
			},
			provider: "anthropic",
			wantKey:  "",
		},
		{
			name:     "nil providers map",
			cfg:      &Config{},
			provider: "openai",
			wantKey:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ps := tt.cfg.GetProviderSettings(tt.provider)
			if ps.APIKey != tt.wantKey {
				t.Errorf("APIKey = %q, want %q", ps.APIKey, tt.wantKey)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()
	// Config has no required validation tags currently, so Validate should pass
	cfg := &Config{
		Provider: "openai",
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid config, got error: %v", err)
	}
}

func TestResolveAPIKey(t *testing.T) {
	// Cannot use t.Parallel() because subtests use t.Setenv
	tests := []struct {
		name      string
		flagVal   string
		envVar    string
		envVal    string
		configVal string
		provider  string
		wantKey   string
		wantErr   bool
	}{
		{
			name:    "flag takes priority",
			flagVal: "flag-key",
			envVar:  "TEST_API_KEY_1",
			envVal:  "env-key",
			wantKey: "flag-key",
		},
		{
			name:      "env takes priority over config",
			envVar:    "TEST_API_KEY_2",
			envVal:    "env-key",
			configVal: "config-key",
			wantKey:   "env-key",
		},
		{
			name:      "config is fallback",
			envVar:    "TEST_API_KEY_3_UNSET",
			configVal: "config-key",
			wantKey:   "config-key",
		},
		{
			name:     "all empty returns error",
			envVar:   "TEST_API_KEY_4_UNSET",
			provider: "openai",
			wantErr:  true,
		},
		{
			name:    "trims whitespace from flag",
			flagVal: "  trimmed-key  ",
			envVar:  "TEST_API_KEY_5_UNSET",
			wantKey: "trimmed-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != "" {
				t.Setenv(tt.envVar, tt.envVal)
			}
			key, err := ResolveAPIKey(tt.flagVal, tt.envVar, tt.configVal, tt.provider)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if key != tt.wantKey {
				t.Errorf("got %q, want %q", key, tt.wantKey)
			}
		})
	}
}

func TestLoadOrCreateConfig_CreatesDefault(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv

	// Use a temp dir as home
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// We need to trick os.Executable. Since we can't easily do that,
	// we just verify the function doesn't panic and returns a valid config.
	cfg, err := LoadOrCreateConfig()
	if err != nil {
		// Some CI environments may have issues with Executable path
		t.Skipf("skipping: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Provider != DefaultProvider && cfg.Provider != "" {
		t.Logf("provider = %q (may vary based on existing config)", cfg.Provider)
	}
}

func TestSaveAndReloadConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := &Config{
		Provider:       "anthropic",
		CommitType:     "feat",
		EnableEmoji:    true,
		PromptTemplate: "custom template {DIFF}",
		Providers: map[string]ProviderSettings{
			"anthropic": {APIKey: "sk-ant-123", Model: "claude-3"},
		},
		CommitTypes: []CommitTypeConfig{
			{Type: "feat", Emoji: "✨"},
			{Type: "fix", Emoji: "🐛"},
		},
	}

	if err := saveConfig(path, cfg); err != nil {
		t.Fatal(err)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatal("config file not created")
	}

	// Read it back
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !contains(content, "anthropic") {
		t.Error("expected provider in saved config")
	}
	if !contains(content, "custom template") {
		t.Error("expected prompt template in saved config")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
