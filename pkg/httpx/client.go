package httpx

import (
    "context"
    "net/http"
    "net/http/cookiejar"
)

// NewDefaultClient returns an HTTP client suitable for SSE endpoints and APIs
// that dislike transparent compression. It also attaches a CookieJar so servers
// behind anti-bot layers (e.g., Cloudflare) can set session cookies.
func NewDefaultClient() *http.Client {
    jar, _ := cookiejar.New(nil)
    return &http.Client{
        // Timeout is managed by per-request contexts.
        Timeout: 0,
        Transport: &http.Transport{
            // Keep raw bytes; SSE with gzip can be problematic across proxies.
            DisableCompression: true,
        },
        Jar: jar,
    }
}

// EnsureSession performs a lightweight GET on baseURL to allow the server to
// issue cookies or perform other session initialization before a long-lived
// SSE/POST request.
func EnsureSession(ctx context.Context, client *http.Client, baseURL string, extraHeaders map[string]string) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
    if err != nil {
        return
    }
    for k, v := range extraHeaders {
        req.Header.Set(k, v)
    }
    resp, err := client.Do(req)
    if err == nil && resp != nil && resp.Body != nil {
        _ = resp.Body.Close()
    }
}

