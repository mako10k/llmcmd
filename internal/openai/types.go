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

// Usage represents token usage information with detailed breakdown
type Usage struct {
	PromptTokens        int                  `json:"prompt_tokens"`
	CompletionTokens    int                  `json:"completion_tokens"`
	TotalTokens         int                  `json:"total_tokens"`
	PromptTokensDetails *PromptTokensDetails `json:"prompt_tokens_details,omitempty"`
}

// PromptTokensDetails represents detailed breakdown of prompt tokens
type PromptTokensDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
}

// QuotaConfig represents quota configuration for token usage
type QuotaConfig struct {
	MaxTokens    int     `json:"max_tokens"`    // Maximum total weighted tokens allowed
	InputWeight  float64 `json:"input_weight"`  // Weight for input tokens (e.g., 1.0 for gpt-4o)
	CachedWeight float64 `json:"cached_weight"` // Weight for cached tokens (e.g., 0.25 for gpt-4o)
	OutputWeight float64 `json:"output_weight"` // Weight for output tokens (e.g., 4.0 for gpt-4o)
}

// QuotaUsage tracks weighted token usage against quota
type QuotaUsage struct {
	InputTokens     int     `json:"input_tokens"`     // Non-cached input tokens
	CachedTokens    int     `json:"cached_tokens"`    // Cached input tokens
	OutputTokens    int     `json:"output_tokens"`    // Output/completion tokens
	WeightedInputs  float64 `json:"weighted_inputs"`  // Input tokens × input weight
	WeightedCached  float64 `json:"weighted_cached"`  // Cached tokens × cached weight
	WeightedOutputs float64 `json:"weighted_outputs"` // Output tokens × output weight
	TotalWeighted   float64 `json:"total_weighted"`   // Sum of all weighted tokens
	RemainingQuota  float64 `json:"remaining_quota"`  // Remaining quota capacity
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

// ClientStats tracks API usage statistics with quota support
type ClientStats struct {
	RequestCount     int           `json:"request_count"`
	TotalTokens      int           `json:"total_tokens"`
	PromptTokens     int           `json:"prompt_tokens"`
	CompletionTokens int           `json:"completion_tokens"`
	TotalDuration    time.Duration `json:"total_duration"`
	LastRequestTime  time.Time     `json:"last_request_time"`
	ErrorCount       int           `json:"error_count"`
	RetryCount       int           `json:"retry_count"`
	QuotaUsage       QuotaUsage    `json:"quota_usage"`    // Quota tracking
	QuotaExceeded    bool          `json:"quota_exceeded"` // Whether quota was exceeded
	Verbose          bool          `json:"-"`              // Not serialized
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
	s.QuotaUsage = QuotaUsage{}
	s.QuotaExceeded = false
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

// UpdateQuotaUsage updates quota usage based on the provided usage data and config
func (s *ClientStats) UpdateQuotaUsage(usage *Usage, config *QuotaConfig) {
	if config == nil || usage == nil {
		return
	}

	// Calculate actual input tokens (subtract cached from total input)
	actualInputTokens := usage.PromptTokens
	cachedTokens := 0
	if usage.PromptTokensDetails != nil {
		cachedTokens = usage.PromptTokensDetails.CachedTokens
		actualInputTokens -= cachedTokens
	}

	// Update token counts
	s.QuotaUsage.InputTokens += actualInputTokens
	s.QuotaUsage.CachedTokens += cachedTokens
	s.QuotaUsage.OutputTokens += usage.CompletionTokens

	// Calculate weighted costs
	s.QuotaUsage.WeightedInputs = float64(s.QuotaUsage.InputTokens) * config.InputWeight
	s.QuotaUsage.WeightedCached = float64(s.QuotaUsage.CachedTokens) * config.CachedWeight
	s.QuotaUsage.WeightedOutputs = float64(s.QuotaUsage.OutputTokens) * config.OutputWeight

	// Calculate total weighted usage
	s.QuotaUsage.TotalWeighted = s.QuotaUsage.WeightedInputs + s.QuotaUsage.WeightedCached + s.QuotaUsage.WeightedOutputs

	// Calculate remaining quota
	if config.MaxTokens <= 0 {
		// No limit set - unlimited quota
		s.QuotaUsage.RemainingQuota = -1 // Indicates unlimited
		s.QuotaExceeded = false
	} else {
		s.QuotaUsage.RemainingQuota = float64(config.MaxTokens) - s.QuotaUsage.TotalWeighted
		// Check if quota exceeded
		s.QuotaExceeded = s.QuotaUsage.RemainingQuota <= 0
	}
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
				Description: "Execute shell scripts using the full shell execution environment. Supports complete shell syntax including pipes, redirects, and complex commands. Pattern 1: spawn({script}) returns new file descriptors. Pattern 2: spawn({script,in_fd}) reads from existing fd. Pattern 3: spawn({script,out_fd}) writes to existing fd. Pattern 4: spawn({script,in_fd,out_fd}) for pipeline middle.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"script": map[string]interface{}{
							"type":        "string",
							"description": "Shell script/command to execute. Supports full shell syntax: pipes (|), redirects (>, >>), command substitution, etc. Examples: 'grep ERROR | sort', 'ls -la *.log | wc -l', 'cat file1 file2 | sort > output'",
						},
						"in_fd": map[string]interface{}{
							"type":        "integer",
							"description": "Input file descriptor for script (optional). When provided with out_fd, runs synchronously.",
							"minimum":     0,
						},
						"out_fd": map[string]interface{}{
							"type":        "integer",
							"description": "Output file descriptor for script (optional). When provided with in_fd, runs synchronously.",
							"minimum":     1,
						},
					},
					"required": []string{"script"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "open",
				Description: "Open virtual files for read/write operations. Creates virtual file descriptors that can be used with read/write tools. Useful for temporary file operations and data storage.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Virtual file path to open",
						},
						"mode": map[string]interface{}{
							"type":        "string",
							"description": "File mode: 'r' (read), 'w' (write), 'a' (append), 'r+' (read/write), 'w+' (write/read), 'a+' (append/read)",
							"enum":        []string{"r", "w", "a", "r+", "w+", "a+"},
							"default":     "r",
						},
					},
					"required": []string{"path"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "close",
				Description: "Close file descriptor and cleanup associated pipeline chains. Required for explicit pipeline endpoint control.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"fd": map[string]interface{}{
							"type":        "integer",
							"description": "File descriptor to close (0=stdin, 1=stdout, 2=stderr, 3+=intermediate fds)",
							"minimum":     0,
						},
					},
					"required": []string{"fd"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "help",
				Description: "Get comprehensive usage information for specific tool categories. Provides detailed guidance, examples, and best practices organized by subsections.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"keys": map[string]interface{}{
							"type":        "array",
							"description": "Usage categories to retrieve: data_analysis, text_processing, file_operations, content_search, format_conversion, log_analysis, batch_processing, interactive_workflow, debugging, basic_operations, command_usage",
							"items": map[string]interface{}{
								"type": "string",
								"enum": []string{
									"data_analysis",
									"text_processing",
									"file_operations",
									"content_search",
									"format_conversion",
									"log_analysis",
									"batch_processing",
									"interactive_workflow",
									"debugging",
									"basic_operations",
									"command_usage",
								},
							},
							"minItems": 1,
							"maxItems": 11,
						},
					},
					"required": []string{"keys"},
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

// ExitToolDefinition returns only the exit tool definition for final API calls
func ExitToolDefinition() []Tool {
	return []Tool{
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
