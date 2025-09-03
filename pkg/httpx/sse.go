package httpx

import (
    "bufio"
    "context"
    "encoding/json"
    "io"
    "strings"
)

// ChunkDecoder extracts a text delta from an SSE data payload.
// It returns (delta, done, ok).
//  - delta: text to append to the aggregate output
//  - done:  whether the stream signaled completion
//  - ok:    whether this payload was recognized/consumed
type ChunkDecoder func(data []byte) (delta string, done bool, ok bool)

// StreamAggregate reads text/event-stream content from r, calls decode for each
// `data:` line, and aggregates the text deltas until completion or EOF.
func StreamAggregate(ctx context.Context, r io.Reader, decode ChunkDecoder) (string, error) {
    scanner := bufio.NewScanner(r)
    // Increase buffer to accommodate larger SSE chunks.
    const maxBuf = 1024 * 1024
    buf := make([]byte, 0, 64*1024)
    scanner.Buffer(buf, maxBuf)

    var out strings.Builder
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        select {
        case <-ctx.Done():
            return out.String(), ctx.Err()
        default:
        }
        if line == "" || !strings.HasPrefix(line, "data: ") {
            continue
        }
        payload := strings.TrimPrefix(line, "data: ")
        if payload == "[DONE]" {
            break
        }
        if delta, done, ok := decode([]byte(payload)); ok {
            if delta != "" {
                out.WriteString(delta)
            }
            if done {
                break
            }
        }
    }
    if err := scanner.Err(); err != nil {
        // Return partial output with error; caller may still use partial text.
        return out.String(), err
    }
    return out.String(), nil
}

// OpenAIStyleDecoder decodes typical OpenAI-like SSE chunks where the payload
// is a JSON object with `choices[0].delta.content` and optional `type:"metadata"`.
func OpenAIStyleDecoder(data []byte) (string, bool, bool) {
    var sr struct {
        Type    string `json:"type"`
        Choices []struct {
            Delta struct {
                Content string `json:"content"`
            } `json:"delta"`
            FinishReason *string `json:"finish_reason"`
        } `json:"choices"`
    }
    if err := json.Unmarshal(data, &sr); err != nil {
        return "", false, false
    }
    if sr.Type == "metadata" {
        return "", false, true
    }
    if len(sr.Choices) == 0 {
        return "", false, true
    }
    delta := sr.Choices[0].Delta.Content
    done := sr.Choices[0].FinishReason != nil && *sr.Choices[0].FinishReason != ""
    return delta, done, true
}

