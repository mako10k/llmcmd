package openai

import (
	"time"
)

// ChatCompletionRequest represents an OpenAI ChatCompletion API request
type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Tools       []Tool        `json:"tools,omitempty"`
	ToolChoice  interface{}   `json:"tool_choice,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
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
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
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
	RetryCount       int           `json:"retry_count"`
	Verbose          bool          `json:"-"` // Not serialized
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
	s.RetryCount = 0
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
						"lines": map[string]interface{}{
							"type":        "integer",
							"description": "Number of lines to read (alternative to count, default: 40)",
							"minimum":     1,
							"maximum":     1000,
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
						"newline": map[string]interface{}{
							"type":        "boolean",
							"description": "Add newline at the end (default: false)",
						},
						"eof": map[string]interface{}{
							"type":        "boolean",
							"description": "Signal end of file and trigger chain cleanup (default: false)",
						},
					},
					"required": []string{"fd", "data"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "spawn",
				Description: "Spawn built-in commands in background mode: 1) spawn({cmd,args}) for new fds, 2) spawn({cmd,args,in_fd,size}) for input, 3) spawn({cmd,args,out_fd}) for output. Use write({eof:true}) to trigger chain cleanup.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"cmd": map[string]interface{}{
							"type":        "string",
							"description": "Command name to execute",
							"enum":        []string{"cat", "grep", "sed", "head", "tail", "sort", "wc", "tr", "cut", "uniq", "nl", "rev"},
						},
						"args": map[string]interface{}{
							"type":        "array",
							"description": "Command arguments array",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
						"in_fd": map[string]interface{}{
							"type":        "integer",
							"description": "Input file descriptor for command (optional). When provided with out_fd, runs synchronously.",
							"minimum":     0,
						},
						"out_fd": map[string]interface{}{
							"type":        "integer",
							"description": "Output file descriptor for command (optional). When provided with in_fd, runs synchronously.",
							"minimum":     1,
						},
						"size": map[string]interface{}{
							"type":        "integer",
							"description": "Number of bytes to process from in_fd (optional). For foreground execution control.",
							"minimum":     1,
						},
					},
					"required": []string{"cmd"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "tee",
				Description: "Copy input from one fd to multiple output fds (1:many relationship). Creates dependency that requires all output fds to be closed before input fd.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"in_fd": map[string]interface{}{
							"type":        "integer",
							"description": "Source file descriptor to read from",
							"minimum":     0,
						},
						"out_fds": map[string]interface{}{
							"type":        "array",
							"description": "Array of destination file descriptors (1=stdout, 2=stderr, or other fds)",
							"items": map[string]interface{}{
								"type":    "integer",
								"minimum": 1,
							},
							"minItems": 1,
						},
					},
					"required": []string{"in_fd", "out_fds"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "exit",
				Description: "Exit the program with specified code",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"code": map[string]interface{}{
							"type":        "integer",
							"description": "Exit code",
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
