package phind

import (
    "bufio"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "strings"

    "github.com/renatogalera/ai-commit/pkg/ai"
    "github.com/renatogalera/ai-commit/pkg/httpx"
)

type PhindClient struct {
	ai.BaseAIClient
	client     *http.Client
	model      string
	apiBaseURL string
	token      string
}

func NewPhindClient(provider, token, model, baseURL string) (*PhindClient, error) {
    if strings.TrimSpace(model) == "" {
        return nil, fmt.Errorf("phind model is required")
    }
    if strings.TrimSpace(baseURL) == "" {
        return nil, fmt.Errorf("phind baseURL is required")
    }
    return &PhindClient{
        BaseAIClient: ai.BaseAIClient{Provider: provider},
        client:      httpx.NewDefaultClient(),
        model:      model,
        apiBaseURL: baseURL,
        token:      token,
    }, nil
}

func (p *PhindClient) GetCommitMessage(ctx context.Context, prompt string) (string, error) {
    // Best-effort: ensure CF sets a session cookie before the heavy POST.
    if u, err := url.Parse(p.apiBaseURL); err == nil {
        if p.client.Jar != nil && len(p.client.Jar.Cookies(u)) == 0 {
            headers := map[string]string{
                "Accept":          "*/*",
                "Accept-Encoding": "Identity",
                "User-Agent":      "",
            }
            if strings.TrimSpace(p.token) != "" {
                headers["Authorization"] = "Bearer " + p.token
            }
            httpx.EnsureSession(ctx, p.client, p.apiBaseURL, headers)
        }
    }
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
    // Match Phind browser/extension headers as closely as possible.
    req.Header.Set("Accept", "*/*")
    req.Header.Set("Accept-Encoding", "Identity")
    // Intentionally blank UA to mimic extension behavior.
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

    if resp.StatusCode != http.StatusOK {
        // Attempt to read any small error body (no assumption of SSE here).
        var errResp struct {
            Error struct {
                Message string `json:"message"`
            } `json:"error"`
        }
        // Best-effort: read up to 1MB from error body
        data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
        bodyStr := string(data)
        _ = json.Unmarshal([]byte(bodyStr), &errResp)
        if errResp.Error.Message != "" {
            return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, errResp.Error.Message)
        }
        return "", fmt.Errorf("unexpected response (status %d): %s", resp.StatusCode, bodyStr)
    }

    // Stream-parse SSE using reusable helper and OpenAI-like decoder.
    text, _ := httpx.StreamAggregate(ctx, resp.Body, httpx.OpenAIStyleDecoder)
    if strings.TrimSpace(text) == "" {
        return "", fmt.Errorf("no completion choice received")
    }
    return text, nil
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

// StreamCommitMessage implements incremental SSE streaming for Phind and emits
// text deltas via onDelta as they arrive. It returns the final aggregated text.
func (p *PhindClient) StreamCommitMessage(ctx context.Context, prompt string, onDelta func(string)) (string, error) {
    // Preflight session/cookies if needed
    if u, err := url.Parse(p.apiBaseURL); err == nil {
        if p.client.Jar != nil && len(p.client.Jar.Cookies(u)) == 0 {
            headers := map[string]string{
                "Accept":          "*/*",
                "Accept-Encoding": "Identity",
                "User-Agent":      "",
            }
            if strings.TrimSpace(p.token) != "" {
                headers["Authorization"] = "Bearer " + p.token
            }
            httpx.EnsureSession(ctx, p.client, p.apiBaseURL, headers)
        }
    }

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
    if strings.TrimSpace(p.token) != "" {
        req.Header.Set("Authorization", "Bearer "+p.token)
    }

    resp, err := p.client.Do(req)
    if err != nil {
        return "", fmt.Errorf("HTTP request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
        var errResp struct{ Error struct{ Message string `json:"message"` } `json:"error"` }
        _ = json.Unmarshal(data, &errResp)
        if errResp.Error.Message != "" {
            return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, errResp.Error.Message)
        }
        return "", fmt.Errorf("unexpected response (status %d): %s", resp.StatusCode, string(data))
    }

    // Stream SSE, emit deltas, and aggregate final content
    type delta struct{ Content string `json:"content"` }
    type choice struct{
        Delta delta `json:"delta"`
        FinishReason *string `json:"finish_reason"`
    }
    type streamResp struct{
        Type string `json:"type"`
        Choices []choice `json:"choices"`
    }

    var out strings.Builder
    scanner := bufio.NewScanner(resp.Body)
    // Increase buffer for safety
    const maxBuf = 1024 * 1024
    buf := make([]byte, 0, 64*1024)
    scanner.Buffer(buf, maxBuf)

    for scanner.Scan() {
        // honor ctx cancellation
        select { case <-ctx.Done(): return out.String(), ctx.Err(); default: }
        line := strings.TrimSpace(scanner.Text())
        if line == "" || !strings.HasPrefix(line, "data: ") { continue }
        payload := strings.TrimPrefix(line, "data: ")
        if payload == "[DONE]" { break }
        var sr streamResp
        if err := json.Unmarshal([]byte(payload), &sr); err != nil { continue }
        if sr.Type == "metadata" { continue }
        if len(sr.Choices) == 0 { continue }
        d := sr.Choices[0].Delta.Content
        if d != "" {
            out.WriteString(d)
            onDelta(d)
        }
        if sr.Choices[0].FinishReason != nil && *sr.Choices[0].FinishReason != "" { break }
    }
    if err := scanner.Err(); err != nil {
        if strings.TrimSpace(out.String()) != "" { return out.String(), nil }
        return "", fmt.Errorf("stream read error: %w", err)
    }
    final := strings.TrimSpace(out.String())
    if final == "" { return "", fmt.Errorf("no completion choice received") }
    return final, nil
}
var _ ai.StreamingAIClient = (*PhindClient)(nil)
