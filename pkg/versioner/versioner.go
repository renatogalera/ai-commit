package versioner

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"golang.org/x/mod/semver"

	"github.com/renatogalera/ai-commit/pkg/ai" // Import AI Client Interface
	// Still using OpenAI utils for prompt - can abstract later
)

// GetCurrentVersionTag recupera a tag Git mais recente que corresponde ao versionamento semântico usando go-git.
func GetCurrentVersionTag(ctx context.Context) (string, error) {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return "", fmt.Errorf("falha ao abrir repositório: %w", err)
	}
	tagIter, err := repo.Tags()
	if err != nil {
		return "", fmt.Errorf("falha ao obter tags: %w", err)
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

// SuggestNextVersion usa OpenAI para sugerir a próxima versão semântica com base na mensagem de commit.
func SuggestNextVersion(ctx context.Context, currentVersion, commitMsg string, client ai.AIClient) (string, error) { // Use AI Client Interface
	if currentVersion == "" {
		currentVersion = "v0.0.0"
	}
	prompt := buildVersionPrompt(currentVersion, commitMsg)
	aiResponse, err := client.GetCommitMessage(ctx, prompt) // Use AI Client Interface
	if err != nil {
		return "", fmt.Errorf("falha ao obter sugestão de versão: %w", err)
	}

	suggested, err := parseAiVersionSuggestion(aiResponse, currentVersion)
	if err != nil {
		return "", fmt.Errorf("falha ao analisar sugestão de versão AI: %w", err)
	}
	return suggested, nil
}

func CreateLocalTag(ctx context.Context, newVersionTag string) error {
	if newVersionTag == "" {
		return errors.New("nenhuma tag de versão fornecida")
	}

	repo, err := git.PlainOpen(".")
	if err != nil {
		return fmt.Errorf("falha ao abrir repositório: %w", err)
	}

	headRef, err := repo.Head()
	if err != nil {
		return fmt.Errorf("falha ao obter referência HEAD: %w", err)
	}

	_, err = repo.CreateTag(newVersionTag, headRef.Hash(), nil)
	if err != nil {
		return fmt.Errorf("falha ao criar tag %s: %w", newVersionTag, err)
	}

	return nil
}

func buildVersionPrompt(currentVersion, commitMsg string) string {
	return fmt.Sprintf(`
Estamos a usar versionamento semântico, onde uma versão é definida como MAJOR.MINOR.PATCH.
A versão atual é %s.
A última mensagem de commit é:
"%s"

Com base na mensagem de commit, determine se a próxima versão é:
- atualização MAJOR se introduzir alterações interruptivas
- atualização MINOR se adicionar novas funcionalidades
- atualização PATCH se for uma correção ou pequena melhoria

Por favor, apresente a próxima versão no formato vX.Y.Z sem explicação extra.
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
