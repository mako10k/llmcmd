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
	"strings"
	"time"
)

// Client represents an OpenAI API client
type Client struct {
	httpClient   *http.Client
	apiKey       string
	baseURL      string
	stats        ClientStats
	maxCalls     int
	retryConfig  RetryConfig
	quotaConfig  *QuotaConfig // Optional quota configuration
	sharedQuota  *SharedQuotaManager // Optional shared quota manager
	processID    string              // Process ID for shared quota
}

// ClientConfig holds configuration for the OpenAI client
type ClientConfig struct {
	APIKey      string
	BaseURL     string
	Timeout     time.Duration
	MaxCalls    int
	MaxRetries  int
	RetryDelay  time.Duration
	QuotaConfig *QuotaConfig // Optional quota configuration
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
		apiKey:      config.APIKey,
		baseURL:     config.BaseURL,
		maxCalls:    config.MaxCalls,
		quotaConfig: config.QuotaConfig,
		retryConfig: RetryConfig{
			MaxRetries:    config.MaxRetries,
			BaseDelay:     config.RetryDelay,
			MaxDelay:      30 * time.Second,
			BackoffFactor: 2.0,
		},
	}
}

// NewClientWithSharedQuota creates a new OpenAI API client with shared quota management
func NewClientWithSharedQuota(config ClientConfig, sharedQuota *SharedQuotaManager, processID string) *Client {
	client := NewClient(config)
	client.sharedQuota = sharedQuota
	client.processID = processID
	return client
}

// errorf is a helper to add error stats and return a formatted error
func (c *Client) errorf(format string, args ...interface{}) (*ChatCompletionResponse, error) {
	c.stats.AddError()
	return nil, fmt.Errorf(format, args...)
}

// ChatCompletion sends a chat completion request to OpenAI API
func (c *Client) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	// Check rate limits
	if c.stats.RequestCount >= c.maxCalls {
		return c.errorf("maximum API calls exceeded (%d/%d)", c.stats.RequestCount, c.maxCalls)
	}

	// Check quota limits (only if limits are set)
	if c.quotaConfig != nil && c.quotaConfig.MaxTokens > 0 && c.stats.QuotaExceeded {
		return c.errorf("quota limit exceeded: %.1f/%.0f weighted tokens used",
			c.stats.QuotaUsage.TotalWeighted, float64(c.quotaConfig.MaxTokens))
	}

	// Prepare request
	reqBody, err := json.Marshal(req)
	if err != nil {
		c.stats.AddError()
		return c.errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		c.stats.AddError()
		return c.errorf("failed to create request: %w", err)
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
		return c.errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.errorf("failed to read response: %w", err)
	}

	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		var errorResp ErrorResponse
		if err := json.Unmarshal(respBody, &errorResp); err != nil {
			return c.errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
		}
		return c.errorf("API error: %s (type: %s)", errorResp.Error.Message, errorResp.Error.Type)
	}

	// Parse successful response
	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return c.errorf("failed to unmarshal response: %w", err)
	}

	// Update statistics
	c.stats.AddRequest(duration, chatResp.Usage)

	// Update quota usage if quota config is provided
	if c.quotaConfig != nil {
		c.stats.UpdateQuotaUsage(&chatResp.Usage, c.quotaConfig)
	}

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
	fileName := filepath.Base(filePath)

	// Check for compound extensions first
	if strings.HasSuffix(fileName, ".tar.gz") || strings.HasSuffix(fileName, ".tar.bz2") || strings.HasSuffix(fileName, ".tar.xz") {
		info["file_type"] = "archive"
	} else {
		switch ext {
		case ".txt", ".md", ".log":
			info["file_type"] = "text"
		case ".json", ".xml", ".yaml", ".yml":
			info["file_type"] = "structured_text"
		case ".csv", ".tsv":
			info["file_type"] = "tabular_data"
		case ".tar", ".tgz", ".zip", ".rar", ".gz", ".bz2", ".xz", ".7z":
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
	defer func() {
		if r := recover(); r != nil {
			// If anything panics, return a safe default
		}
	}()

	// Try to get file info for the standard FD
	var stat os.FileInfo
	var err error

	// Use different approaches based on the FD
	switch fd {
	case 0: // stdin
		stat, err = os.Stdin.Stat()
	case 1: // stdout
		stat, err = os.Stdout.Stat()
	case 2: // stderr
		stat, err = os.Stderr.Stat()
	default:
		// For other FDs, try os.NewFile approach
		file := os.NewFile(uintptr(fd), fmt.Sprintf("fd%d", fd))
		if file == nil {
			return map[string]interface{}{
				"type": "terminal",
			}
		}
		defer file.Close()
		stat, err = file.Stat()
	}

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

		// Try to get the actual file path if possible (Linux/Unix only)
		fdPath := fmt.Sprintf("/proc/self/fd/%d", fd)
		if realPath, err := os.Readlink(fdPath); err == nil {
			info["file_path"] = realPath

			// Get file type from extension
			ext := filepath.Ext(realPath)
			fileName := filepath.Base(realPath)

			// Check for compound extensions first
			if strings.HasSuffix(fileName, ".tar.gz") || strings.HasSuffix(fileName, ".tar.bz2") || strings.HasSuffix(fileName, ".tar.xz") {
				info["file_type"] = "archive"
			} else {
				switch ext {
				case ".txt", ".md", ".log":
					info["file_type"] = "text"
				case ".json", ".xml", ".yaml", ".yml":
					info["file_type"] = "structured_text"
				case ".csv", ".tsv":
					info["file_type"] = "tabular_data"
				case ".tar", ".tgz", ".zip", ".rar", ".gz", ".bz2", ".xz", ".7z":
					info["file_type"] = "archive"
				case ".bin", ".exe", ".so", ".dll":
					info["file_type"] = "binary"
				case ".jpg", ".jpeg", ".png", ".gif", ".bmp":
					info["file_type"] = "image"
				default:
					info["file_type"] = "unknown"
				}
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
		} else {
			// If we can't get the path, still provide basic file info
			info["file_type"] = "unknown"
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
	return CreateInitialMessagesWithQuota(prompt, instructions, inputFiles, customSystemPrompt, disableTools, "", false)
}

// CreateInitialMessagesWithQuota creates the initial message sequence with quota information
func CreateInitialMessagesWithQuota(prompt, instructions string, inputFiles []string, customSystemPrompt string, disableTools bool, quotaStatus string, isLastCall bool) []ChatMessage {
	var messages []ChatMessage

	// Use custom system prompt if provided, otherwise use default
	var systemContent string
	if customSystemPrompt != "" {
		systemContent = customSystemPrompt
	} else if disableTools {
		// Simple system message when tools are disabled
		systemContent = `You are a helpful assistant. Provide direct, clear answers to user questions without using any special tools or functions. Generate your response directly as plain text.`
	} else {
		// Fixed system prompt with llmsh overview and tool descriptions
		systemContent = `You are llmcmd, an expert command-line text processing assistant that operates within the llmsh shell environment.

🏠 LLMSH INTEGRATION:
llmsh is an LLM-powered shell that provides intelligent command execution and file processing capabilities. You (llmcmd) serve as the core text processing engine, handling complex data manipulation tasks through a secure tool interface.

🔧 AVAILABLE TOOLS:

1. read(fd, [lines], [count]) - Read content for processing
   • fd: 0=stdin, 3+=input files  
   • lines/count: optional limits
   • Use for: Data analysis, content examination

2. write(fd, data, [newline], [eof]) - Output data
   • fd: 1=stdout, 2=stderr, or command input
   • eof: true signals end-of-input (important for commands)
   • Use for: Final output, command input

3. open(path, [mode]) - Open virtual files
   • path: file path to open
   • mode: "r"(read), "w"(write), "a"(append), "r+"(read/write), "w+"(write/read), "a+"(append/read)
   • Returns: assigned file descriptor for use with read/write
   • PIPE behavior: files can only be read ONCE, then they're consumed
   • Use for: Creating temporary files, managing virtual file operations

4. spawn(script, [in_fd], [out_fd]) - Execute shell scripts
   • Full shell script execution environment
   • Returns immediately with file descriptors
   • Patterns: spawn(script) → {in_fd, out_fd}, spawn(script, in_fd) → {out_fd}
   • Scripts run in background - use read() to get results
   • Full shell syntax: pipes (|), redirects (>, >>), command substitution
   • Examples: spawn("grep ERROR | sort"), spawn("ls *.log | wc -l")

5. close(fd) - Close file descriptors and cleanup chains

6. exit(code) - Terminate (0=success, 1=error)

🛠️ SHELL EXECUTION ENVIRONMENT:
• Built-in text processing commands (cat, grep, sed, head, tail, sort, wc, tr)
• No external commands (ls, find, awk, python, etc.) - use spawn() for shell scripts only
• Full shell syntax: pipes, redirects, command substitution within spawn()
• Integrated with llmsh for intelligent command interpretation

📋 STANDARD WORKFLOWS:

A) Simple Processing:
   read(0) → process data → write(1, result) → exit(0)

B) Shell Command Processing:
   spawn(script) → write(in_fd, data, {eof:true}) → read(out_fd) → write(1, result) → exit(0)

C) Virtual File Operations:
   open("temp.txt", "w") → get fd → write(fd, data) → read from files → exit(0)

D) File Analysis (without reading content):
   • For binary files (.pdf, .jpg, .zip): Answer based on filename/extension
   • For unknown files: Use file size/name clues, avoid reading binary data

E) Text Content Analysis:
   • Use read(fd) for text files
   • Apply shell commands for processing

🎯 EFFICIENCY PRINCIPLES:
1. Minimize API calls - combine operations intelligently
2. Choose appropriate tools for the task
3. Use open() for virtual file management
4. Always signal EOF with write({eof:true}) for commands
5. Read command output with read() for LLM processing
6. Handle errors gracefully and provide clear feedback

🔄 SHELL COMMAND EXECUTION PATTERN:
For any command needing input/output:
1. spawn(script) → get {in_fd, out_fd}
2. write(in_fd, input_data, {eof:true}) → send data to command
3. read(out_fd) → get command results for processing
4. Process results and write(1, final_output)
5. exit(0)

💾 VIRTUAL FILE PATTERN:
For temporary file operations with PIPE-like behavior:
1. open("filename", "w") → get fd for writing
2. write(fd, data) → store data in virtual file
3. open("filename", "r") → get fd for reading (only works once!)
4. read(fd) → retrieve stored data (file becomes consumed after full read)
5. Process and output results
⚠️  IMPORTANT: Virtual files act like PIPES - once fully read, they cannot be read again unless recreated

Remember: You are the intelligent core of llmsh, providing efficient text processing with full shell environment capabilities!`
	}

	// Add special instructions for last API call
	if isLastCall && !disableTools {
		systemContent += "\n\n⚠️  FINAL API CALL - MUST EXIT:\nThis is your final API call. You MUST use the exit() tool to terminate the program. Only the exit tool is available. Provide a completion summary if appropriate, then call exit(0) for success or exit(1) for errors."
	}

	messages = append(messages, ChatMessage{
		Role:    "system",
		Content: systemContent,
	})

	// Skip FD mapping and technical details if tools are disabled
	if disableTools {
		// For disabled tools, just add the user instruction directly
		userContent := instructions
		if prompt != "" {
			if userContent != "" {
				userContent = prompt + "\n\n" + userContent
			} else {
				userContent = prompt
			}
		}

		if userContent != "" {
			messages = append(messages, ChatMessage{
				Role:    "user",
				Content: userContent,
			})
		}

		return messages
	}

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
		fdMappingContent += "\nWORKFLOW: read(fd=3+) → spawn(commands) → write(fd=1) → exit(0)"
		fdMappingContent += "\n\nFILE REFERENCES: Use $1 for first file, $2 for second file, etc."
	} else {
		fdMappingContent += "\n\nAVAILABLE INPUT SOURCES:"
		if stdinInfo["type"] == "file" {
			fdMappingContent += "\n✓ stdin (fd=0) - redirected from file, contains input data to process"
		} else {
			fdMappingContent += "\n✓ stdin (fd=0) - contains input data"
		}
		fdMappingContent += "\n✗ input files - none specified (do NOT read fd=3+)"
		fdMappingContent += "\nWORKFLOW: read(fd=0) → spawn(commands) → write(fd=1) → exit(0)"
	}

	messages = append(messages, ChatMessage{
		Role:    "user",
		Content: fdMappingContent,
	})

	// Second user message: User's actual prompt/instructions with quota status
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

	// Add quota status information to the last message if provided
	if quotaStatus != "" {
		userContent += "\n\nCURRENT USAGE STATUS:\n" + quotaStatus
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
