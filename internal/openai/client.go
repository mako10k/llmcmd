package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Client represents an OpenAI API client
type Client struct {
	httpClient  *http.Client
	apiKey      string
	baseURL     string
	stats       ClientStats
	maxCalls    int
	retryConfig RetryConfig
}

// ClientConfig holds configuration for the OpenAI client
type ClientConfig struct {
	APIKey     string
	BaseURL    string
	Timeout    time.Duration
	MaxCalls   int
	MaxRetries int
	RetryDelay time.Duration
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
			MaxRetries:    config.MaxRetries,
			BaseDelay:     config.RetryDelay,
			MaxDelay:      30 * time.Second,
			BackoffFactor: 2.0,
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

// getFileInfo retrieves file information for display in prompts
func getFileInfo(filePath string) map[string]interface{} {
	stat, err := os.Stat(filePath)
	if err != nil {
		return map[string]interface{}{
			"name":  filepath.Base(filePath),
			"error": fmt.Sprintf("cannot access: %v", err),
		}
	}

	// Check if this is a regular file that can be safely read without consuming streams
	mode := stat.Mode()
	if !mode.IsRegular() {
		// Not a regular file (pipe, device, socket, etc.) - don't read metadata
		return map[string]interface{}{
			"name":          filepath.Base(filePath),
			"file_type":     "stream/device",
			"size_category": "unknown",
			"stream_note":   "non-regular file - content not pre-read",
		}
	}

	// Additional check: try to open and seek to verify it's seekable
	file, err := os.Open(filePath)
	if err != nil {
		return map[string]interface{}{
			"name":  filepath.Base(filePath),
			"error": fmt.Sprintf("cannot open: %v", err),
		}
	}
	defer file.Close()

	// Test if file is seekable (won't work on pipes, devices, etc.)
	_, err = file.Seek(0, 0)
	if err != nil {
		return map[string]interface{}{
			"name":          filepath.Base(filePath),
			"file_type":     "non-seekable",
			"size_category": "unknown",
			"stream_note":   "file not seekable - likely a stream or special device",
		}
	}

	info := map[string]interface{}{
		"name":       filepath.Base(filePath),
		"path":       filePath,
		"size_bytes": stat.Size(),
		"mode":       stat.Mode().String(),
		"modtime":    stat.ModTime().Format("2006-01-02 15:04:05"),
		"isdir":      stat.IsDir(),
	}

	// Determine file type based on extension and content
	ext := filepath.Ext(filePath)
	switch ext {
	case ".txt", ".md", ".log":
		info["file_type"] = "text"
	case ".json", ".xml", ".yaml", ".yml":
		info["file_type"] = "structured_text"
	case ".csv", ".tsv":
		info["file_type"] = "tabular_data"
	case ".tar", ".tar.gz", ".tgz", ".zip", ".rar":
		info["file_type"] = "archive"
	case ".bin", ".exe", ".so", ".dll":
		info["file_type"] = "binary"
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp":
		info["file_type"] = "image"
	case ".mp3", ".wav", ".ogg":
		info["file_type"] = "audio"
	case ".mp4", ".avi", ".mkv":
		info["file_type"] = "video"
	default:
		if stat.Mode()&0111 != 0 {
			info["file_type"] = "executable"
		} else {
			info["file_type"] = "unknown"
		}
	}

	// Size category for better understanding
	size := stat.Size()
	if size < 1024 {
		info["size_category"] = "small"
	} else if size < 1024*1024 {
		info["size_category"] = "medium"
	} else if size < 10*1024*1024 {
		info["size_category"] = "large"
	} else {
		info["size_category"] = "very_large"
	}

	return info
}

// getStdFileInfo gets file information for standard file descriptors (stdin/stdout/stderr)
func getStdFileInfo(fd int) map[string]interface{} {
	// Try to get file info for the standard FD
	file := os.NewFile(uintptr(fd), fmt.Sprintf("fd%d", fd))
	if file == nil {
		return map[string]interface{}{
			"type": "terminal",
		}
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return map[string]interface{}{
			"type": "terminal",
		}
	}

	info := map[string]interface{}{
		"mode": stat.Mode().String(),
	}

	// Check if it's a regular file (redirected from/to a file)
	if stat.Mode().IsRegular() {
		// It's a regular file - get full file information
		info["size_bytes"] = stat.Size()
		info["modtime"] = stat.ModTime().Format("2006-01-02 15:04:05")
		info["type"] = "file"

		// Try to get the actual file path if possible
		if fdPath := fmt.Sprintf("/proc/self/fd/%d", fd); true {
			if realPath, err := os.Readlink(fdPath); err == nil {
				info["file_path"] = realPath
				
				// Get file type from extension
				ext := filepath.Ext(realPath)
				switch ext {
				case ".txt", ".md", ".log":
					info["file_type"] = "text"
				case ".json", ".xml", ".yaml", ".yml":
					info["file_type"] = "structured_text"
				case ".csv", ".tsv":
					info["file_type"] = "tabular_data"
				case ".tar", ".tar.gz", ".tgz", ".zip", ".rar":
					info["file_type"] = "archive"
				case ".bin", ".exe", ".so", ".dll":
					info["file_type"] = "binary"
				case ".jpg", ".jpeg", ".png", ".gif", ".bmp":
					info["file_type"] = "image"
				default:
					info["file_type"] = "unknown"
				}

				// Size category
				size := stat.Size()
				if size < 1024 {
					info["size_category"] = "small"
				} else if size < 1024*1024 {
					info["size_category"] = "medium"
				} else if size < 10*1024*1024 {
					info["size_category"] = "large"
				} else {
					info["size_category"] = "very_large"
				}
			}
		}
	} else if stat.Mode()&os.ModeCharDevice != 0 {
		info["type"] = "terminal"
	} else if stat.Mode()&os.ModeNamedPipe != 0 {
		info["type"] = "pipe"
	} else {
		info["type"] = "other"
	}

	return info
}

// CreateInitialMessages creates the initial message sequence for llmcmd
func CreateInitialMessages(prompt, instructions string, inputFiles []string, customSystemPrompt string, disableTools bool) []ChatMessage {
	var messages []ChatMessage

	// Use custom system prompt if provided, otherwise use default
	var systemContent string
	if customSystemPrompt != "" {
		systemContent = customSystemPrompt
	} else if disableTools {
		// Simple system message when tools are disabled
		systemContent = `You are a helpful assistant. Provide direct, clear answers to user questions without using any special tools or functions. Generate your response directly as plain text.`
	} else {
		// Default system message with tool descriptions and efficiency guidelines
		systemContent = `You are a command-line text processing assistant. Process user requests efficiently using these tools:

TOOLS AVAILABLE:
1. read(fd) - Read from file descriptors (count=bytes, lines=line count)
2. write(fd, data) - Write to stdout/stderr (newline=true adds \\n)
3. pipe(commands, input) - Execute built-in commands
4. exit(code) - Terminate program

STANDARD WORKFLOW:
1. read(fd=0) for stdin → 2. process data → 3. write(fd=1, data) → 4. exit(0)

EXAMPLE WORKFLOW:
For line processing: read(fd=0, lines=40) for efficient reading
For filtering and sorting: pipe({commands:[{name:"grep",args:["apple"]},{name:"sort",args:["-u"]}], input:{type:"fd",fd:0}})
For final output: write(fd=1, data, newline=true) to ensure proper formatting

EFFICIENCY GUIDELINES:
- Use minimal API calls - combine operations when possible
- Read data in appropriate chunks
- Process streaming data efficiently

IMPORTANT: Analyze INPUT TEXT from stdin, not the question language. Provide direct answers about the input data.`
	}

	messages = append(messages, ChatMessage{
		Role:    "system",
		Content: systemContent,
	})

	// First user message: Technical file descriptor information
	var fdMappingContent string
	var actualFiles []string
	for _, file := range inputFiles {
		if file != "-" {
			actualFiles = append(actualFiles, file)
		}
	}

	fdMappingContent = "FILE DESCRIPTOR MAPPING:"
	
	// Check stdin information
	stdinInfo := getStdFileInfo(0)
	stdinDisplay := "stdin (standard input)"
	if stdinInfo["type"] == "file" {
		if filePath, ok := stdinInfo["file_path"].(string); ok {
			size := stdinInfo["size_bytes"].(int64)
			sizeStr := ""
			if size < 1024 {
				sizeStr = fmt.Sprintf("%d bytes", size)
			} else if size < 1024*1024 {
				sizeStr = fmt.Sprintf("%.1f KB", float64(size)/1024)
			} else if size < 1024*1024*1024 {
				sizeStr = fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
			} else {
				sizeStr = fmt.Sprintf("%.1f GB", float64(size)/(1024*1024*1024))
			}
			
			fileType := "unknown"
			if ftype, ok := stdinInfo["file_type"].(string); ok {
				fileType = ftype
			}
			
			sizeCategory := "unknown"
			if category, ok := stdinInfo["size_category"].(string); ok {
				sizeCategory = category
			}
			
			stdinDisplay = fmt.Sprintf("stdin <- %s [%s, %s, %s]", filePath, sizeStr, fileType, sizeCategory)
		}
	}
	
	// Check stdout information
	stdoutInfo := getStdFileInfo(1)
	stdoutDisplay := "stdout (standard output - write results here)"
	if stdoutInfo["type"] == "file" {
		if filePath, ok := stdoutInfo["file_path"].(string); ok {
			stdoutDisplay = fmt.Sprintf("stdout -> %s", filePath)
		}
	}
	
	// Check stderr information
	stderrInfo := getStdFileInfo(2)
	stderrDisplay := "stderr (error output)"
	if stderrInfo["type"] == "file" {
		if filePath, ok := stderrInfo["file_path"].(string); ok {
			stderrDisplay = fmt.Sprintf("stderr -> %s", filePath)
		}
	}
	
	fdMappingContent += fmt.Sprintf("\n- fd=0: %s", stdinDisplay)
	fdMappingContent += fmt.Sprintf("\n- fd=1: %s", stdoutDisplay)
	fdMappingContent += fmt.Sprintf("\n- fd=2: %s", stderrDisplay)

	if len(actualFiles) > 0 {
		for i, file := range actualFiles {
			// Get file information for pre-loading
			fileInfo := getFileInfo(file)

			var infoDisplay string

			// Check if it's a stream device
			if streamNote, isStream := fileInfo["stream_note"].(string); isStream {
				infoDisplay = fmt.Sprintf("[%s]", streamNote)
			} else if errorMsg, hasError := fileInfo["error"].(string); hasError {
				infoDisplay = fmt.Sprintf("[%s]", errorMsg)
			} else {
				// Regular file - show size, type, category
				sizeStr := "unknown size"
				if size, ok := fileInfo["size_bytes"].(int64); ok {
					if size < 1024 {
						sizeStr = fmt.Sprintf("%d bytes", size)
					} else if size < 1024*1024 {
						sizeStr = fmt.Sprintf("%.1f KB", float64(size)/1024)
					} else if size < 1024*1024*1024 {
						sizeStr = fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
					} else {
						sizeStr = fmt.Sprintf("%.1f GB", float64(size)/(1024*1024*1024))
					}
				}

				fileType := "unknown"
				if ftype, ok := fileInfo["file_type"].(string); ok {
					fileType = ftype
				}

				sizeCategory := "unknown"
				if category, ok := fileInfo["size_category"].(string); ok {
					sizeCategory = category
				}

				infoDisplay = fmt.Sprintf("[%s, %s, %s]", sizeStr, fileType, sizeCategory)
			}

			fdMappingContent += fmt.Sprintf("\n- fd=%d: %s (input file #%d) %s",
				i+3, file, i+1, infoDisplay)
		}
		fdMappingContent += "\n\nAVAILABLE INPUT SOURCES:"
		fdMappingContent += "\n✓ input files (fd=3+) - specified above, contains data to process"
		if stdinInfo["type"] == "file" {
			fdMappingContent += "\n? stdin (fd=0) - redirected from file, may also contain data"
		} else {
			fdMappingContent += "\n✗ stdin (fd=0) - ignore, no input data here"
		}
		fdMappingContent += "\nWORKFLOW: read(fd=3+) → pipe(commands) → write(fd=1) → exit(0)"
		fdMappingContent += "\n\nFILE REFERENCES: Use $1 for first file, $2 for second file, etc."
	} else {
		fdMappingContent += "\n\nAVAILABLE INPUT SOURCES:"
		if stdinInfo["type"] == "file" {
			fdMappingContent += "\n✓ stdin (fd=0) - redirected from file, contains input data to process"
		} else {
			fdMappingContent += "\n✓ stdin (fd=0) - contains input data"
		}
		fdMappingContent += "\n✗ input files - none specified (do NOT read fd=3+)"
		fdMappingContent += "\nWORKFLOW: read(fd=0) → pipe(commands) → write(fd=1) → exit(0)"
	}

	messages = append(messages, ChatMessage{
		Role:    "user",
		Content: fdMappingContent,
	})

	// Second user message: User's actual prompt/instructions
	var userContent string
	if len(actualFiles) > 0 {
		// ファイルがある場合はファイル参照の説明を追加
		fileRefs := "\n\nFILE REFERENCES:"
		for i := range actualFiles {
			fileRefs += fmt.Sprintf("\n- $%d = input file #%d", i+1, i+1)
		}
		fileRefs += "\n- stdin/stdout/stderr = standard streams"

		if prompt != "" && instructions != "" {
			userContent = fmt.Sprintf("Process the input files according to this request:\n\nPrompt: %s\n\nInstructions: %s%s", prompt, instructions, fileRefs)
		} else if prompt != "" {
			userContent = fmt.Sprintf("Process the input files according to this request:\n\n%s%s", prompt, fileRefs)
		} else {
			userContent = fmt.Sprintf("Process the input files according to this request:\n\n%s%s", instructions, fileRefs)
		}
	} else {
		// 標準入力の場合
		if prompt != "" && instructions != "" {
			userContent = fmt.Sprintf("Process the input data from stdin according to this request:\n\nPrompt: %s\n\nInstructions: %s", prompt, instructions)
		} else if prompt != "" {
			userContent = fmt.Sprintf("Process the input data from stdin according to this request:\n\n%s", prompt)
		} else {
			userContent = fmt.Sprintf("Process the input data from stdin according to this request:\n\n%s", instructions)
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
