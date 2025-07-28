package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client represents an OpenAI API client
type Client struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
	stats      ClientStats
	maxCalls   int
}

// ClientConfig holds configuration for the OpenAI client
type ClientConfig struct {
	APIKey     string
	BaseURL    string
	Timeout    time.Duration
	MaxCalls   int
}

// NewClient creates a new OpenAI API client
func NewClient(config ClientConfig) *Client {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.openai.com/v1"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxCalls == 0 {
		config.MaxCalls = 50
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		apiKey:   config.APIKey,
		baseURL:  config.BaseURL,
		maxCalls: config.MaxCalls,
	}
}

// ChatCompletion sends a chat completion request to OpenAI API
func (c *Client) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	// Check rate limits
	if c.stats.RequestCount >= c.maxCalls {
		return nil, fmt.Errorf("maximum API calls exceeded (%d/%d)", c.stats.RequestCount, c.maxCalls)
	}

	// Prepare request
	reqBody, err := json.Marshal(req)
	if err != nil {
		c.stats.AddError()
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		c.stats.AddError()
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("User-Agent", "llmcmd/1.0.0")

	// Send request and measure duration
	start := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	duration := time.Since(start)

	if err != nil {
		c.stats.AddError()
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.stats.AddError()
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		var errorResp ErrorResponse
		if err := json.Unmarshal(respBody, &errorResp); err != nil {
			c.stats.AddError()
			return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
		}
		c.stats.AddError()
		return nil, fmt.Errorf("API error: %s (type: %s)", errorResp.Error.Message, errorResp.Error.Type)
	}

	// Parse successful response
	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		c.stats.AddError()
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Update statistics
	c.stats.AddRequest(duration, chatResp.Usage)

	return &chatResp, nil
}

// GetStats returns current client statistics
func (c *Client) GetStats() ClientStats {
	return c.stats
}

// ResetStats resets client statistics
func (c *Client) ResetStats() {
	c.stats.Reset()
}

// CreateInitialMessages creates the initial message sequence for llmcmd
func CreateInitialMessages(prompt, instructions string, inputFiles []string) []ChatMessage {
	var messages []ChatMessage

	// System message with tool descriptions
	systemContent := `You are a command-line text processing assistant. You have access to these tools:

1. read(fd, count=4096) - Read data from file descriptors:
   - fd=0: stdin
   - fd=3+: input files (in order specified)

2. write(fd, data) - Write data to output:
   - fd=1: stdout
   - fd=2: stderr

3. pipe(command, args=[], input_fd=0, output_fd=1) - Execute built-in commands:
   - cat: Copy data
   - grep: Pattern matching
   - sed: Text substitution  
   - head/tail: Line filtering
   - sort: Alphabetical sorting
   - wc: Count lines/words/characters
   - tr: Character translation

4. exit(code, message="") - Terminate with exit code

Process the user's request step by step. Always use exit(0) when complete.
Security: Only built-in commands are available - no external command execution.`

	messages = append(messages, ChatMessage{
		Role:    "system",
		Content: systemContent,
	})

	// User message with prompt and instructions
	var userContent string
	if prompt != "" && instructions != "" {
		userContent = fmt.Sprintf("Prompt: %s\n\nInstructions: %s", prompt, instructions)
	} else if prompt != "" {
		userContent = prompt
	} else {
		userContent = instructions
	}

	// Add input file information
	if len(inputFiles) > 0 {
		userContent += "\n\nAvailable input files:"
		for i, file := range inputFiles {
			userContent += fmt.Sprintf("\n- fd=%d: %s", i+3, file)
		}
	}

	messages = append(messages, ChatMessage{
		Role:    "user",
		Content: userContent,
	})

	return messages
}

// CreateToolResponseMessage creates a message from tool execution results
func CreateToolResponseMessage(toolCallID, result string) ChatMessage {
	return ChatMessage{
		Role:    "tool",
		Content: result,
		ToolCalls: []ToolCall{
			{
				ID:   toolCallID,
				Type: "function",
			},
		},
	}
}
