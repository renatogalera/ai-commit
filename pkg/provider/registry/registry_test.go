package registry

import (
	"context"
	"sort"
	"sync"
	"testing"

	"github.com/renatogalera/ai-commit/pkg/ai"
	"github.com/renatogalera/ai-commit/pkg/config"
)

// resetRegistry clears global state for isolated tests.
// Must NOT be used in parallel tests.
func resetRegistry() {
	mu.Lock()
	factories = map[string]Factory{}
	defaults = map[string]config.ProviderSettings{}
	required = map[string]bool{}
	mu.Unlock()
}

func dummyFactory(_ context.Context, _ string, _ config.ProviderSettings) (ai.AIClient, error) {
	return nil, nil
}

func TestRegisterAndGet(t *testing.T) {
	resetRegistry()

	Register("testprovider", dummyFactory)

	f, ok := Get("testprovider")
	if !ok {
		t.Fatal("expected provider to be registered")
	}
	if f == nil {
		t.Fatal("expected non-nil factory")
	}

	_, ok = Get("nonexistent")
	if ok {
		t.Error("expected false for unregistered provider")
	}
}

func TestHas(t *testing.T) {
	resetRegistry()

	Register("myprovider", dummyFactory)

	if !Has("myprovider") {
		t.Error("expected Has to return true")
	}
	if Has("other") {
		t.Error("expected Has to return false for unregistered")
	}
}

func TestNames(t *testing.T) {
	resetRegistry()

	Register("alpha", dummyFactory)
	Register("beta", dummyFactory)
	Register("gamma", dummyFactory)

	names := Names()
	sort.Strings(names)

	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
	want := []string{"alpha", "beta", "gamma"}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("names[%d] = %q, want %q", i, n, want[i])
		}
	}
}

func TestRegisterDefaults(t *testing.T) {
	resetRegistry()

	ps := config.ProviderSettings{
		Model:   "gpt-4",
		BaseURL: "https://api.openai.com/v1",
	}
	RegisterDefaults("openai", ps)

	got, ok := GetDefaults("openai")
	if !ok {
		t.Fatal("expected defaults to be registered")
	}
	if got.Model != "gpt-4" {
		t.Errorf("Model = %q, want gpt-4", got.Model)
	}
	if got.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("BaseURL = %q", got.BaseURL)
	}

	_, ok = GetDefaults("nonexistent")
	if ok {
		t.Error("expected false for unregistered defaults")
	}
}

func TestRequiresAPIKey(t *testing.T) {
	resetRegistry()

	SetRequiresAPIKey("openai", true)
	SetRequiresAPIKey("ollama", false)

	if !RequiresAPIKey("openai") {
		t.Error("expected openai to require API key")
	}
	if RequiresAPIKey("ollama") {
		t.Error("expected ollama to NOT require API key")
	}
	if RequiresAPIKey("unknown") {
		t.Error("expected unknown provider to return false")
	}
}

func TestConcurrentAccess(t *testing.T) {
	resetRegistry()

	var wg sync.WaitGroup
	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := "provider" + string(rune('A'+i%26))
			Register(name, dummyFactory)
			RegisterDefaults(name, config.ProviderSettings{Model: "model"})
			SetRequiresAPIKey(name, i%2 == 0)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := "provider" + string(rune('A'+i%26))
			Has(name)
			Get(name)
			Names()
			GetDefaults(name)
			RequiresAPIKey(name)
		}(i)
	}

	wg.Wait()
	// If we get here without a race condition, the test passes
}

func TestOverwriteFactory(t *testing.T) {
	resetRegistry()

	Register("prov", dummyFactory)
	if !Has("prov") {
		t.Fatal("expected registered")
	}

	// Overwrite with new factory
	newFactory := func(_ context.Context, _ string, _ config.ProviderSettings) (ai.AIClient, error) {
		return nil, nil
	}
	Register("prov", newFactory)

	f, ok := Get("prov")
	if !ok || f == nil {
		t.Error("expected overwritten factory to exist")
	}
}
