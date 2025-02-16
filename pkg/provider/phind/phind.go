package phind

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/renatogalera/ai-commit/pkg/ai"
)

type PhindClient struct {
	ai.BaseAIClient
	client     *http.Client
	model      string
	apiBaseURL string
	token      string
}

func NewPhindClient(token, model string) *PhindClient {
	if model == "" {
		model = "Phind-70B"
	}
	return &PhindClient{
		BaseAIClient: ai.BaseAIClient{Provider: "phind"},
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		model:      model,
		apiBaseURL: "https://https.extension.phind.com/agent/",
		token:      token,
	}
}

func (p *PhindClient) GetCommitMessage(ctx context.Context, prompt string) (string, error) {
	payload := map[string]interface{}{
		"additional_extension_context": "",
		"allow_magic_buttons":          true,
		"is_vscode_extension":          true,
		"message_history": []map[string]string{
			{
				"content": prompt,
				"role":    "user",
			},
		},
		"requested_model": p.model,
		"user_input":      prompt,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.apiBaseURL, strings.NewReader(string(data)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "Identity")
	req.Header.Set("User-Agent", "")

	// Se o token for fornecido, inclui no header de autorização
	if strings.TrimSpace(p.token) != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}
	bodyStr := string(bodyBytes)

	if resp.StatusCode == http.StatusOK {
		completeText := parseStreamResponse(bodyStr)
		if strings.TrimSpace(completeText) == "" {
			return "", fmt.Errorf("no completion choice received")
		}
		return completeText, nil
	}

	// Caso o status não seja 200, tenta extrair a mensagem de erro do JSON
	var errResp struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(bodyBytes, &errResp); err != nil || errResp.Error.Message == "" {
		return "", fmt.Errorf("unexpected response (status %d): %s", resp.StatusCode, bodyStr)
	}

	return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, errResp.Error.Message)
}

func parseStreamResponse(responseText string) string {
	var fullText strings.Builder
	lines := strings.Split(responseText, "\n")

	// Define structs auxiliares para decodificar o JSON de cada linha
	type delta struct {
		Content string `json:"content"`
	}
	type choice struct {
		Delta delta `json:"delta"`
	}
	type streamResp struct {
		Choices []choice `json:"choices"`
	}

	for _, line := range lines {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		var sr streamResp
		if err := json.Unmarshal([]byte(data), &sr); err != nil {
			continue
		}
		if len(sr.Choices) > 0 {
			fullText.WriteString(sr.Choices[0].Delta.Content)
		}
	}
	return fullText.String()
}

func (p *PhindClient) SanitizeResponse(message, commitType string) string {
	return p.BaseAIClient.SanitizeResponse(message, commitType)
}

func (p *PhindClient) ProviderName() string {
	return p.BaseAIClient.Provider
}

func (p *PhindClient) MaybeSummarizeDiff(diff string, maxLength int) (string, bool) {
	return p.BaseAIClient.MaybeSummarizeDiff(diff, maxLength)
}
