package openai

import (
	"time"
)

// ChatCompletionRequest represents an OpenAI ChatCompletion API request
type ChatCompletionRequest struct {
	Model       string                 `json:"model"`
	Messages    []ChatMessage          `json:"messages"`
	Tools       []Tool                 `json:"tools,omitempty"`
	ToolChoice  interface{}            `json:"tool_choice,omitempty"`
	MaxTokens   int                    `json:"max_tokens,omitempty"`
	Temperature float64                `json:"temperature,omitempty"`
	Stream      bool                   `json:"stream,omitempty"`
}

// ChatCompletionResponse represents an OpenAI ChatCompletion API response
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// ChatMessage represents a chat message
type ChatMessage struct {
	Role      string     `json:"role"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// Choice represents a choice in the response
type Choice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Tool represents a function tool definition
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction represents a function definition
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToolCall represents a tool call from the assistant
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction represents function call details
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ErrorResponse represents an error response from OpenAI API
type ErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code,omitempty"`
	} `json:"error"`
}

// ClientStats tracks API usage statistics
type ClientStats struct {
	RequestCount     int           `json:"request_count"`
	TotalTokens      int           `json:"total_tokens"`
	PromptTokens     int           `json:"prompt_tokens"`
	CompletionTokens int           `json:"completion_tokens"`
	TotalDuration    time.Duration `json:"total_duration"`
	LastRequestTime  time.Time     `json:"last_request_time"`
	ErrorCount       int           `json:"error_count"`
}

// Reset resets the statistics
func (s *ClientStats) Reset() {
	s.RequestCount = 0
	s.TotalTokens = 0
	s.PromptTokens = 0
	s.CompletionTokens = 0
	s.TotalDuration = 0
	s.LastRequestTime = time.Time{}
	s.ErrorCount = 0
}

// AddRequest updates statistics with a new request
func (s *ClientStats) AddRequest(duration time.Duration, usage Usage) {
	s.RequestCount++
	s.TotalTokens += usage.TotalTokens
	s.PromptTokens += usage.PromptTokens
	s.CompletionTokens += usage.CompletionTokens
	s.TotalDuration += duration
	s.LastRequestTime = time.Now()
}

// AddError increments error count
func (s *ClientStats) AddError() {
	s.ErrorCount++
}

// ToolDefinitions returns the standard tool definitions for llmcmd
func ToolDefinitions() []Tool {
	return []Tool{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "read",
				Description: "Read data from a file descriptor or stream",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"fd": map[string]interface{}{
							"type":        "integer",
							"description": "File descriptor number (0=stdin, 3+=input files)",
							"minimum":     0,
						},
						"count": map[string]interface{}{
							"type":        "integer",
							"description": "Number of bytes to read (max 4096)",
							"minimum":     1,
							"maximum":     4096,
						},
					},
					"required": []string{"fd"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "write",
				Description: "Write data to a file descriptor or stream",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"fd": map[string]interface{}{
							"type":        "integer",
							"description": "File descriptor number (1=stdout, 2=stderr)",
							"minimum":     1,
							"maximum":     2,
						},
						"data": map[string]interface{}{
							"type":        "string",
							"description": "Data to write",
						},
					},
					"required": []string{"fd", "data"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "pipe",
				Description: "Execute a built-in command with input/output redirection",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]interface{}{
							"type":        "string",
							"description": "Built-in command name (cat, grep, sed, head, tail, sort, wc, tr)",
							"enum":        []string{"cat", "grep", "sed", "head", "tail", "sort", "wc", "tr"},
						},
						"args": map[string]interface{}{
							"type":        "array",
							"description": "Command arguments",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
						"input_fd": map[string]interface{}{
							"type":        "integer",
							"description": "Input file descriptor (0=stdin, 3+=input files)",
							"minimum":     0,
						},
						"output_fd": map[string]interface{}{
							"type":        "integer",
							"description": "Output file descriptor (1=stdout, 2=stderr)",
							"minimum":     1,
							"maximum":     2,
						},
					},
					"required": []string{"command"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "exit",
				Description: "Terminate the program with an exit code",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"code": map[string]interface{}{
							"type":        "integer",
							"description": "Exit code (0=success, 1-255=error)",
							"minimum":     0,
							"maximum":     255,
						},
						"message": map[string]interface{}{
							"type":        "string",
							"description": "Optional exit message",
						},
					},
					"required": []string{"code"},
				},
			},
		},
	}
}
