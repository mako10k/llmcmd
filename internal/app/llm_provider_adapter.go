package app

import (
    "context"
    "fmt"

    "github.com/mako10k/llmcmd/internal/llm"
    "github.com/mako10k/llmcmd/internal/openai"
)

// openAIProvider adapts internal/openai.Client to llm.Provider
type openAIProvider struct { client *openai.Client }

func newOpenAIProvider(c *openai.Client) *openAIProvider { return &openAIProvider{client: c} }

func (p *openAIProvider) Chat(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
    if p.client == nil { return llm.ChatResponse{}, fmt.Errorf("openai client nil") }
    // Map llm request to OpenAI request
    messages := make([]openai.ChatMessage, 0, len(req.Messages))
    for _, m := range req.Messages {
        messages = append(messages, openai.ChatMessage{Role: m.Role, Content: m.Content})
    }
    oreq := openai.ChatCompletionRequest{
        Model:       req.Model,
        Messages:    messages,
        MaxTokens:   req.MaxTokens,
        Temperature: req.Temperature,
    }
    ores, err := p.client.ChatCompletion(ctx, oreq)
    if err != nil { return llm.ChatResponse{}, err }
    var content string
    if len(ores.Choices) > 0 {
        content = ores.Choices[0].Message.Content
    }
    resp := llm.ChatResponse{
        Messages: []llm.Message{{Role: "assistant", Content: content}},
        Usage: llm.Usage{
            PromptTokens:     ores.Usage.PromptTokens,
            CompletionTokens: ores.Usage.CompletionTokens,
            TotalTokens:      ores.Usage.TotalTokens,
        },
        Model:    oreq.Model,
        Metadata: map[string]any{},
    }
    return resp, nil
}
