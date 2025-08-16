package app

import (
    "io"
    "net/http"
    "strings"
    "time"

    "github.com/mako10k/llmcmd/internal/openai"
)

// roundTripperFunc allows using a function as http.RoundTripper
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// newMockLLMClient creates an OpenAI client that returns a canned chat response
func newMockLLMClient() *openai.Client {
    client := openai.NewClient(openai.ClientConfig{
        APIKey:   "test",
        BaseURL:  "http://localhost",
        Timeout:  5 * time.Second,
        MaxCalls: 10,
    })

    body := `{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`
    httpClient := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
        return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
    })}
    client.OverrideHTTPClient(httpClient)

    return client
}
