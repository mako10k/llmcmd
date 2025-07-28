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
	retryConfig RetryConfig
}

// ClientConfig holds configuration for the OpenAI client
type ClientConfig struct {
	APIKey      string
	BaseURL     string
	Timeout     time.Duration
	MaxCalls    int
	MaxRetries  int
	RetryDelay  time.Duration
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
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 1 * time.Second
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		apiKey:   config.APIKey,
		baseURL:  config.BaseURL,
		maxCalls: config.MaxCalls,
		retryConfig: RetryConfig{
			MaxRetries:      config.MaxRetries,
			BaseDelay:       config.RetryDelay,
			MaxDelay:        30 * time.Second,
			BackoffFactor:   2.0,
		},
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
func CreateInitialMessages(prompt, instructions string, inputFiles []string, customSystemPrompt string) []ChatMessage {
	var messages []ChatMessage

	// Use custom system prompt if provided, otherwise use default
	var systemContent string
	if customSystemPrompt != "" {
		systemContent = customSystemPrompt
	} else {
		// Default system message with tool descriptions and efficiency guidelines
		systemContent = `You are a helpful command-line text processing assistant.

TOOLS AVAILABLE:
1. read(fd) - Read from file descriptors
2. write(fd, data) - Write to output (fd=1: stdout, fd=2: stderr)  
3. pipe(commands, input) - Execute built-in commands
4. exit(code) - Terminate

STANDARD WORKFLOW:
1. ALWAYS read input data first: read(fd=0) for stdin
2. Process the data according to user's request
3. ALWAYS write results to stdout: write(fd=1, data + "\\n")
4. ALWAYS finish with: exit(0)

IMPORTANT RULES:
- When asked about language/content, analyze the INPUT TEXT from stdin (not the question language)
- The user's question language is different from the input text language
- Example: Input="hello world", Question="これは何語ですか？" → Answer="英語" (because input is English)
- Always provide a clear, direct answer about the INPUT data
- Add newline (\\n) to output for proper formatting
- Never use complex pipeline commands unless specifically needed
- If unsure, provide the most straightforward interpretation

Process the user's request step by step using this workflow.`
	}

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

	// Add input source information with clear file descriptor mapping
	var actualFiles []string
	for _, file := range inputFiles {
		if file != "-" {
			actualFiles = append(actualFiles, file)
		}
	}

	userContent += "\n\nFILE DESCRIPTOR MAPPING:"
	userContent += "\n- fd=0: stdin (standard input)"
	userContent += "\n- fd=1: stdout (standard output - write results here)"
	userContent += "\n- fd=2: stderr (error output)"

	if len(actualFiles) > 0 {
		for i, file := range actualFiles {
			userContent += fmt.Sprintf("\n- fd=%d: %s (input file)", i+3, file)
		}
		userContent += "\n\nAVAILABLE INPUT SOURCES:"
		userContent += "\n✓ stdin (fd=0) - contains input data"
		userContent += "\n✓ input files (fd=3+) - specified above"
		userContent += "\nWORKFLOW: read(fd=0 or fd=3+) → pipe(commands) → write(fd=1) → exit(0)"
	} else {
		userContent += "\n\nAVAILABLE INPUT SOURCES:"
		userContent += "\n✓ stdin (fd=0) - contains input data"
		userContent += "\n✗ input files - none specified (do NOT read fd=3+)"
		userContent += "\nWORKFLOW: read(fd=0) → pipe(commands) → write(fd=1) → exit(0)"
	}

	messages = append(messages, ChatMessage{
		Role:    "user",
		Content: userContent,
	})

	return messages
}

// CreateToolResponseMessage creates a message from tool execution results
func CreateToolResponseMessage(toolCallID, result string) ChatMessage {
	// Ensure content is never empty to avoid OpenAI API errors
	content := result
	if content == "" {
		content = "(no output)"
	}
	
	return ChatMessage{
		Role:       "tool",
		Content:    content,
		ToolCallID: toolCallID,
	}
}

// SetVerbose enables or disables verbose logging
func (c *Client) SetVerbose(verbose bool) {
	c.stats.Verbose = verbose
}
