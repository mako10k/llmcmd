package llmsh

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/mako10k/llmcmd/internal/openai"
)

// LLMRecursiveExecutor handles recursive llmcmd execution with quota management
type LLMRecursiveExecutor struct {
	client       *openai.Client
	quotaManager *openai.SharedQuotaManager
	processID    string
	parentID     string
}

// NewLLMRecursiveExecutor creates a new recursive executor
func NewLLMRecursiveExecutor(client *openai.Client, quotaManager *openai.SharedQuotaManager, parentID string) *LLMRecursiveExecutor {
	processID := fmt.Sprintf("llmcmd-%d", time.Now().UnixNano())

	executor := &LLMRecursiveExecutor{
		client:       client,
		quotaManager: quotaManager,
		processID:    processID,
		parentID:     parentID,
	}

	// Register this process with the quota manager
	if quotaManager != nil {
		quotaManager.RegisterProcess(processID, parentID)
	}

	return executor
}

// Execute executes a prompt with LLM and manages quota
func (e *LLMRecursiveExecutor) Execute(prompt string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	// Check quota before execution
	if e.quotaManager != nil && !e.quotaManager.CanMakeCall(e.processID) {
		return fmt.Errorf("quota exceeded: cannot make LLM call")
	}

	// Read input data if available
	var inputData string
	if stdin != nil {
		input, err := io.ReadAll(stdin)
		if err != nil {
			return fmt.Errorf("error reading input: %w", err)
		}
		inputData = string(input)
	}

	// Construct full prompt
	fullPrompt := e.constructPrompt(prompt, inputData)

	// Create chat completion request
	request := openai.ChatCompletionRequest{
		Model: "gpt-4o-mini", // Fixed model for recursive calls
		Messages: []openai.ChatMessage{
			{
				Role:    "user",
				Content: fullPrompt,
			},
		},
		MaxTokens:   2000, // Reasonable limit for recursive calls
		Temperature: 0.7,
	}

	// Execute LLM call
	ctx := context.Background()
	response, err := e.client.ChatCompletion(ctx, request)
	if err != nil {
		return fmt.Errorf("LLM API call failed: %w", err)
	}

	// Update quota usage
	if e.quotaManager != nil {
		usage := &openai.QuotaUsage{
			InputTokens:  response.Usage.PromptTokens,
			OutputTokens: response.Usage.CompletionTokens,
		}

		// Handle cached tokens if available
		if response.Usage.PromptTokensDetails.CachedTokens > 0 {
			usage.CachedTokens = response.Usage.PromptTokensDetails.CachedTokens
			usage.InputTokens -= usage.CachedTokens
		}

		err = e.quotaManager.ConsumeTokens(e.processID, usage)
		if err != nil {
			return fmt.Errorf("quota update failed: %w", err)
		}
	}

	// Output result
	if len(response.Choices) > 0 {
		result := response.Choices[0].Message.Content
		_, err = stdout.Write([]byte(result))
		if err != nil {
			return fmt.Errorf("error writing output: %w", err)
		}
	}

	return nil
}

// constructPrompt builds the full prompt from user prompt and input data
func (e *LLMRecursiveExecutor) constructPrompt(userPrompt, inputData string) string {
	if inputData != "" {
		return fmt.Sprintf(`You are a text processing assistant. You receive input data and a task description, then provide the requested output.

Input Data:
%s

Task: %s

Provide only the requested output without additional explanation.`, inputData, userPrompt)
	}

	return fmt.Sprintf(`You are a text processing assistant. Process the following request:

%s

Provide only the requested output without additional explanation.`, userPrompt)
}

// Cleanup unregisters the process from quota management
func (e *LLMRecursiveExecutor) Cleanup() error {
	if e.quotaManager != nil {
		return e.quotaManager.UnregisterProcess(e.processID)
	}
	return nil
}

// GetProcessID returns the process ID for this executor
func (e *LLMRecursiveExecutor) GetProcessID() string {
	return e.processID
}

// GetQuotaUsage returns the current quota usage for this process
func (e *LLMRecursiveExecutor) GetQuotaUsage() (*openai.QuotaUsage, error) {
	if e.quotaManager != nil {
		return e.quotaManager.GetProcessUsage(e.processID)
	}
	return nil, fmt.Errorf("quota manager not available")
}
