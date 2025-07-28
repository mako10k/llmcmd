package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mako10k/llmcmd/internal/cli"
	"github.com/mako10k/llmcmd/internal/openai"
	"github.com/mako10k/llmcmd/internal/tools"
)

// App represents the main application
type App struct {
	config         *cli.Config
	fileConfig     *cli.ConfigFile
	openaiClient   *openai.Client
	toolEngine     *tools.Engine
	startTime      time.Time
	iterationCount int
	exitRequested  bool
	exitCode       int
}

// New creates a new application instance
func New(config *cli.Config) *App {
	return &App{
		config:    config,
		startTime: time.Now(),
	}
}

// Run executes the main application logic
func (a *App) Run() error {
	// Load configuration file
	var err error
	a.fileConfig, err = cli.LoadConfigFile(a.config.ConfigFile)
	if err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}

	// Apply environment variable overrides
	cli.LoadEnvironmentConfig(a.fileConfig)

	// Validate essential configuration
	if err := a.validateConfig(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	if a.config.Verbose {
		log.Printf("Configuration loaded successfully")
		log.Printf("Config file: %s", a.config.ConfigFile)
		log.Printf("Input files: %v", a.config.InputFiles)
		log.Printf("Output file: %s", a.config.OutputFile)
		log.Printf("Model: %s", a.fileConfig.Model)
		log.Printf("Max API calls: %d", a.fileConfig.MaxAPICalls)
	}

	// Initialize OpenAI client
	if err := a.initializeOpenAI(); err != nil {
		return fmt.Errorf("failed to initialize OpenAI client: %w", err)
	}

	// Initialize tool execution engine
	if err := a.initializeToolEngine(); err != nil {
		return fmt.Errorf("failed to initialize tool engine: %w", err)
	}

	// Execute LLM interaction
	if err := a.executeTask(); err != nil {
		return fmt.Errorf("task execution failed: %w", err)
	}

	// Show statistics if requested
	if a.config.ShowStats {
		a.showStatistics()
	}

	return nil
}

// initializeOpenAI initializes the OpenAI client
func (a *App) initializeOpenAI() error {
	config := openai.ClientConfig{
		APIKey:     a.fileConfig.OpenAIAPIKey,
		BaseURL:    a.fileConfig.OpenAIBaseURL,
		Timeout:    time.Duration(a.fileConfig.TimeoutSeconds) * time.Second,
		MaxCalls:   a.fileConfig.MaxAPICalls,
		MaxRetries: a.fileConfig.MaxRetries,
		RetryDelay: time.Duration(a.fileConfig.RetryDelay) * time.Millisecond,
	}

	a.openaiClient = openai.NewClient(config)
	
	// Enable verbose mode in client stats
	a.openaiClient.SetVerbose(a.config.Verbose)

	if a.config.Verbose {
		log.Printf("OpenAI client initialized (base URL: %s, model: %s)", 
			a.fileConfig.OpenAIBaseURL, a.fileConfig.Model)
	}

	return nil
}

// initializeToolEngine initializes the tool execution engine  
func (a *App) initializeToolEngine() error {
	config := tools.EngineConfig{
		InputFiles:  a.config.InputFiles,
		OutputFile:  a.config.OutputFile,
		MaxFileSize: a.fileConfig.MaxFileSize,
		BufferSize:  a.fileConfig.ReadBufferSize,
	}

	var err error
	a.toolEngine, err = tools.NewEngine(config)
	if err != nil {
		return err
	}

	if a.config.Verbose {
		log.Printf("Tool engine initialized (input files: %d, buffer size: %d)", 
			len(a.config.InputFiles), a.fileConfig.ReadBufferSize)
	}

	return nil
}

// executeTask executes the main LLM task
func (a *App) executeTask() error {
	defer a.toolEngine.Close()

	// Create initial messages
	messages := openai.CreateInitialMessages(
		a.config.Prompt, 
		a.config.Instructions, 
		a.config.InputFiles,
	)

	if a.config.Verbose {
		log.Printf("Starting LLM interaction with %d initial messages", len(messages))
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 
		time.Duration(a.fileConfig.TimeoutSeconds)*time.Second)
	defer cancel()

	// Main interaction loop
	for {
		a.iterationCount++
		
		// Create request
		request := openai.ChatCompletionRequest{
			Model:       a.fileConfig.Model,
			Messages:    messages,
			Tools:       openai.ToolDefinitions(),
			ToolChoice:  "auto",
			MaxTokens:   a.fileConfig.MaxTokens,
			Temperature: a.fileConfig.Temperature,
		}

		// Send request to OpenAI with retry mechanism
		response, err := a.openaiClient.ChatCompletionWithRetry(ctx, request)
		if err != nil {
			return fmt.Errorf("OpenAI API error: %w", err)
		}

		// Process response
		choice := response.Choices[0]
		messages = append(messages, choice.Message)

		if a.config.Verbose {
			stats := a.openaiClient.GetStats()
			log.Printf("API call completed (total: %d/%d, retries: %d, tokens: %d)", 
				stats.RequestCount, a.fileConfig.MaxAPICalls, stats.RetryCount, response.Usage.TotalTokens)
		}

		// Handle finish reason
		switch choice.FinishReason {
		case "stop":
			// Normal completion without tool calls
			if a.config.Verbose {
				log.Printf("LLM completed normally (no tool calls)")
			}
			return nil

		case "tool_calls":
			// Execute tool calls
			if err := a.executeToolCalls(choice.Message.ToolCalls, &messages); err != nil {
				// Check if this is an exit request
				if strings.HasPrefix(err.Error(), "EXIT_REQUESTED:") {
					// Exit was requested, return without error
					return nil
				}
				return fmt.Errorf("tool execution error: %w", err)
			}

		case "length":
			return fmt.Errorf("response truncated due to length limit")

		default:
			return fmt.Errorf("unexpected finish reason: %s", choice.FinishReason)
		}
	}
}

// executeToolCalls executes tool calls and updates messages
func (a *App) executeToolCalls(toolCalls []openai.ToolCall, messages *[]openai.ChatMessage) error {
	if a.config.Verbose {
		log.Printf("Executing %d tool calls", len(toolCalls))
	}

	for _, toolCall := range toolCalls {
		if a.config.Verbose {
			log.Printf("Executing tool: %s (ID: %s)", toolCall.Function.Name, toolCall.ID)
		}

		// Convert to format expected by tool engine
		toolCallMap := map[string]interface{}{
			"name":      toolCall.Function.Name,
			"arguments": toolCall.Function.Arguments,
		}

		// Execute the tool call
		result, err := a.toolEngine.ExecuteToolCall(toolCallMap)
		if err != nil {
			// Check if this is an exit request
			if strings.HasPrefix(err.Error(), "EXIT_REQUESTED:") {
				// Extract exit code
				exitCodeStr := strings.TrimPrefix(err.Error(), "EXIT_REQUESTED:")
				if exitCode, parseErr := strconv.Atoi(exitCodeStr); parseErr == nil {
					a.exitCode = exitCode
					a.exitRequested = true
					// Add tool response to messages
					toolMessage := openai.CreateToolResponseMessage(toolCall.ID, result)
					*messages = append(*messages, toolMessage)
					// Return special error to indicate exit
					return fmt.Errorf("EXIT_REQUESTED:%d", exitCode)
				}
			}
			result = fmt.Sprintf("Error: %v", err)
		}

		// Add tool response to messages
		toolMessage := openai.CreateToolResponseMessage(toolCall.ID, result)
		*messages = append(*messages, toolMessage)

		if a.config.Verbose {
			log.Printf("Tool result: %s", result)
		}
	}

	return nil
}

// GetExitCode returns the exit code requested by exit tool
func (a *App) GetExitCode() int {
	return a.exitCode
}

// IsExitRequested returns whether exit was requested by exit tool  
func (a *App) IsExitRequested() bool {
	return a.exitRequested
}

// validateConfig validates the loaded configuration
func (a *App) validateConfig() error {
	// Check OpenAI API key
	if a.fileConfig.OpenAIAPIKey == "" {
		return fmt.Errorf("OpenAI API key is required. Set it in config file or OPENAI_API_KEY environment variable")
	}

	// Validate model name
	if a.fileConfig.Model == "" {
		return fmt.Errorf("model name is required")
	}

	// Validate numeric ranges
	if a.fileConfig.MaxTokens <= 0 || a.fileConfig.MaxTokens > 32768 {
		return fmt.Errorf("max_tokens must be between 1 and 32768")
	}

	if a.fileConfig.Temperature < 0.0 || a.fileConfig.Temperature > 2.0 {
		return fmt.Errorf("temperature must be between 0.0 and 2.0")
	}

	if a.fileConfig.MaxAPICalls <= 0 || a.fileConfig.MaxAPICalls > 1000 {
		return fmt.Errorf("max_api_calls must be between 1 and 1000")
	}

	if a.fileConfig.TimeoutSeconds <= 0 || a.fileConfig.TimeoutSeconds > 3600 {
		return fmt.Errorf("timeout_seconds must be between 1 and 3600")
	}

	if a.fileConfig.MaxFileSize <= 0 || a.fileConfig.MaxFileSize > 100*1024*1024 {
		return fmt.Errorf("max_file_size must be between 1 and 100MB")
	}

	if a.fileConfig.ReadBufferSize <= 0 || a.fileConfig.ReadBufferSize > 64*1024 {
		return fmt.Errorf("read_buffer_size must be between 1 and 64KB")
	}

	return nil
}

// showStatistics displays detailed execution statistics
func (a *App) showStatistics() {
	duration := time.Since(a.startTime)
	openaiStats := a.openaiClient.GetStats()
	toolStats := a.toolEngine.GetStats()

	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "=== LLMCMD EXECUTION STATISTICS ===\n")
	fmt.Fprintf(os.Stderr, "\n")

	// Timing Information
	fmt.Fprintf(os.Stderr, "⏱️  TIMING:\n")
	fmt.Fprintf(os.Stderr, "   Total Duration:     %v\n", duration.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "   Average per API:    %v\n", (openaiStats.TotalDuration / time.Duration(max(openaiStats.RequestCount, 1))).Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "   LLM Iterations:     %d\n", a.iterationCount)
	fmt.Fprintf(os.Stderr, "\n")

	// OpenAI API Statistics
	fmt.Fprintf(os.Stderr, "🤖 OPENAI API USAGE:\n")
	fmt.Fprintf(os.Stderr, "   API Calls:          %d / %d (%.1f%%)\n", 
		openaiStats.RequestCount, a.fileConfig.MaxAPICalls, 
		float64(openaiStats.RequestCount)/float64(a.fileConfig.MaxAPICalls)*100)
	fmt.Fprintf(os.Stderr, "   Total Retries:      %d\n", openaiStats.RetryCount)
	fmt.Fprintf(os.Stderr, "   Total Tokens:       %d\n", openaiStats.TotalTokens)
	fmt.Fprintf(os.Stderr, "   Prompt Tokens:      %d\n", openaiStats.PromptTokens)
	fmt.Fprintf(os.Stderr, "   Completion Tokens:  %d\n", openaiStats.CompletionTokens)
	fmt.Fprintf(os.Stderr, "   Error Count:        %d\n", openaiStats.ErrorCount)
	if openaiStats.RequestCount > 0 {
		fmt.Fprintf(os.Stderr, "   Avg Tokens/Call:    %.1f\n", float64(openaiStats.TotalTokens)/float64(openaiStats.RequestCount))
	}
	fmt.Fprintf(os.Stderr, "\n")

	// Tool Usage Statistics
	fmt.Fprintf(os.Stderr, "🔧 TOOL USAGE:\n")
	fmt.Fprintf(os.Stderr, "   Read Calls:         %d\n", toolStats.ReadCalls)
	fmt.Fprintf(os.Stderr, "   Write Calls:        %d\n", toolStats.WriteCalls)
	fmt.Fprintf(os.Stderr, "   Pipe Calls:         %d\n", toolStats.PipeCalls)
	fmt.Fprintf(os.Stderr, "   Exit Calls:         %d\n", toolStats.ExitCalls)
	fmt.Fprintf(os.Stderr, "   Total Tool Calls:   %d\n", toolStats.ReadCalls+toolStats.WriteCalls+toolStats.PipeCalls+toolStats.ExitCalls)
	fmt.Fprintf(os.Stderr, "\n")

	// Data Transfer Statistics
	fmt.Fprintf(os.Stderr, "📊 DATA TRANSFER:\n")
	fmt.Fprintf(os.Stderr, "   Bytes Read:         %s\n", formatBytes(toolStats.BytesRead))
	fmt.Fprintf(os.Stderr, "   Bytes Written:      %s\n", formatBytes(toolStats.BytesWritten))
	fmt.Fprintf(os.Stderr, "   Error Count:        %d\n", toolStats.ErrorCount)
	fmt.Fprintf(os.Stderr, "\n")

	// Efficiency Metrics
	if a.iterationCount > 0 && openaiStats.RequestCount > 0 {
		fmt.Fprintf(os.Stderr, "⚡ EFFICIENCY METRICS:\n")
		fmt.Fprintf(os.Stderr, "   API Calls/Iteration: %.2f\n", float64(openaiStats.RequestCount)/float64(a.iterationCount))
		fmt.Fprintf(os.Stderr, "   Tools/API Call:      %.2f\n", float64(toolStats.ReadCalls+toolStats.WriteCalls+toolStats.PipeCalls+toolStats.ExitCalls)/float64(openaiStats.RequestCount))
		
		tokensPerSecond := float64(openaiStats.TotalTokens) / duration.Seconds()
		fmt.Fprintf(os.Stderr, "   Tokens/Second:       %.1f\n", tokensPerSecond)
		
		if toolStats.BytesRead > 0 {
			fmt.Fprintf(os.Stderr, "   Processing Rate:     %s/sec\n", formatBytes(int64(float64(toolStats.BytesRead)/duration.Seconds())))
		}
		fmt.Fprintf(os.Stderr, "\n")
	}

	// Model Information
	fmt.Fprintf(os.Stderr, "🎯 CONFIGURATION:\n")
	fmt.Fprintf(os.Stderr, "   Model:              %s\n", a.fileConfig.Model)
	fmt.Fprintf(os.Stderr, "   Max Tokens:         %d\n", a.fileConfig.MaxTokens)
	fmt.Fprintf(os.Stderr, "   Temperature:        %.1f\n", a.fileConfig.Temperature)
	fmt.Fprintf(os.Stderr, "   Input Files:        %d\n", len(a.config.InputFiles))
	fmt.Fprintf(os.Stderr, "   Buffer Size:        %s\n", formatBytes(int64(a.fileConfig.ReadBufferSize)))
	fmt.Fprintf(os.Stderr, "\n")

	fmt.Fprintf(os.Stderr, "=== END STATISTICS ===\n")
}

// formatBytes formats byte counts in human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
