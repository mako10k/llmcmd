package app

import (
	"context"
	"fmt"
	"io"
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
	a.fileConfig, err = cli.LoadAndMergeConfig(a.config)
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
	if err := a.executeWithError(a.initializeOpenAI, "initialize OpenAI client"); err != nil {
		return err
	}

	// Initialize tool execution engine
	if err := a.executeWithError(a.initializeToolEngine, "initialize tool engine"); err != nil {
		return err
	}

	// Execute LLM interaction
	if err := a.executeWithError(a.executeTask, "execute task"); err != nil {
		return err
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
		QuotaConfig: &openai.QuotaConfig{
			MaxTokens:    a.fileConfig.QuotaMaxTokens,
			InputWeight:  a.fileConfig.QuotaWeights.InputWeight,
			CachedWeight: a.fileConfig.QuotaWeights.InputCachedWeight,
			OutputWeight: a.fileConfig.QuotaWeights.OutputWeight,
		},
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
		NoStdin:     a.config.NoStdin,
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

	// Save configuration on exit (to persist quota usage)
	defer func() {
		if saveErr := a.fileConfig.SaveConfigFile(a.config.ConfigFile); saveErr != nil && a.config.Verbose {
			log.Printf("Warning: failed to save config file: %v", saveErr)
		}
	}()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(a.fileConfig.TimeoutSeconds)*time.Second)
	defer cancel()

	// Create initial messages for first iteration
	quotaStatus := a.fileConfig.GetQuotaStatusString()
	messages := openai.CreateInitialMessagesWithQuota(
		a.config.Prompt,
		a.config.Instructions,
		a.config.InputFiles,
		a.fileConfig.SystemPrompt,
		a.fileConfig.DisableTools,
		quotaStatus,
		false, // Initial call is never the last call
	)

	if a.config.Verbose {
		log.Printf("Starting LLM interaction with %d initial messages", len(messages))
	}

	// Main interaction loop
	for {
		a.iterationCount++

		// Check if this will be the last API call
		stats := a.openaiClient.GetStats()
		isLastCall := (stats.RequestCount + 1) >= a.fileConfig.MaxAPICalls

		// Update quota status for subsequent calls (but preserve message history!)
		if a.iterationCount > 1 {
			quotaStatus = a.fileConfig.GetQuotaStatusString()
			// Update only the system message with quota info, preserving conversation history
			if len(messages) > 0 && messages[0].Role == "system" {
				// Update system message to include quota status
				updatedSystemMessages := openai.CreateInitialMessagesWithQuota(
					a.config.Prompt,
					a.config.Instructions,
					a.config.InputFiles,
					a.fileConfig.SystemPrompt,
					a.fileConfig.DisableTools,
					quotaStatus,
					isLastCall,
				)
				// Replace only the system message, keep all other history
				if len(updatedSystemMessages) > 0 {
					messages[0] = updatedSystemMessages[0]
				}
			}
		}

		// Create request
		request := openai.ChatCompletionRequest{
			Model:       a.fileConfig.Model,
			Messages:    messages,
			MaxTokens:   a.fileConfig.MaxTokens,
			Temperature: a.fileConfig.Temperature,
		}

		// Add tools only if not disabled
		if !a.fileConfig.DisableTools {
			// Use the already calculated isLastCall value
			if isLastCall {
				// Last API call: only provide exit tool and force its use
				request.Tools = openai.ExitToolDefinition()
				request.ToolChoice = map[string]interface{}{
					"type":     "function",
					"function": map[string]string{"name": "exit"},
				}
			} else {
				// Normal API call: provide all tools
				request.Tools = openai.ToolDefinitions()
				request.ToolChoice = "auto"
			}
		}

		// Send request to OpenAI with retry mechanism
		response, err := a.openaiClient.ChatCompletionWithRetry(ctx, request)
		if err != nil {
			return fmt.Errorf("OpenAI API error: %w", err)
		}

		// Process response
		choice := response.Choices[0]
		messages = append(messages, choice.Message)

		// Update quota usage in config file
		actualInputTokens := response.Usage.PromptTokens
		cachedTokens := 0
		if response.Usage.PromptTokensDetails != nil {
			cachedTokens = response.Usage.PromptTokensDetails.CachedTokens
			actualInputTokens -= cachedTokens
		}
		a.fileConfig.UpdateQuotaUsage(actualInputTokens, cachedTokens, response.Usage.CompletionTokens)

		// Sync API call count from client stats
		stats = a.openaiClient.GetStats()
		a.fileConfig.QuotaUsage.APICalls = stats.RequestCount

		// Check for quota exceeded after update
		if a.fileConfig.IsQuotaExceeded() {
			return fmt.Errorf("quota limit exceeded: %s", a.fileConfig.GetQuotaStatusString())
		}

		if a.config.Verbose {
			// Use the already retrieved stats
			log.Printf("API call completed (total: %d/%d, retries: %d, tokens: %d)",
				stats.RequestCount, a.fileConfig.MaxAPICalls, stats.RetryCount, response.Usage.TotalTokens)
			if a.fileConfig.QuotaMaxTokens > 0 {
				log.Printf("Quota status: %s", a.fileConfig.GetQuotaStatusString())
			}
		}

		// Handle finish reason
		switch choice.FinishReason {
		case "stop":
			// Normal completion without tool calls
			if a.config.Verbose {
				log.Printf("LLM completed normally (no tool calls)")
			}

			// Output the LLM response directly when tools are disabled
			if a.fileConfig.DisableTools && choice.Message.Content != "" {
				var output io.Writer
				if a.config.OutputFile != "" {
					// Output file is handled by tool engine, but when tools are disabled,
					// we need to handle it ourselves
					if a.config.OutputFile == "-" {
						output = os.Stdout
					} else {
						file, err := os.Create(a.config.OutputFile)
						if err != nil {
							return fmt.Errorf("failed to create output file: %w", err)
						}
						defer file.Close()
						output = file
					}
				} else {
					output = os.Stdout
				}

				if _, err := output.Write([]byte(choice.Message.Content)); err != nil {
					return fmt.Errorf("failed to write output: %w", err)
				}
			} else if !a.fileConfig.DisableTools && choice.Message.Content != "" {
				// Tools are enabled but LLM returned direct text instead of using tools
				// This is usually an error in LLM behavior - log it in verbose mode
				if a.config.Verbose {
					log.Printf("Warning: LLM returned direct text instead of using tools: %s", choice.Message.Content)
				}
				// Don't output the content as it's likely instruction text, not the actual result
			}

			return nil

		case "tool_calls":
			// Execute tool calls only if tools are enabled
			if a.fileConfig.DisableTools {
				if a.config.Verbose {
					log.Printf("Tool calls requested but tools are disabled")
				}
				return nil
			}

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
			log.Printf("Executing tool: %s (ID: %s) with args: %s",
				toolCall.Function.Name, toolCall.ID, toolCall.Function.Arguments)
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

// executeWithError wraps function execution with standardized error handling
func (a *App) executeWithError(fn func() error, operation string) error {
	if err := fn(); err != nil {
		return fmt.Errorf("failed to %s: %w", operation, err)
	}
	return nil
}

// validateRange validates that a value is within the specified range
func validateRange(value int, min, max int, name string) error {
	if value < min || value > max {
		return fmt.Errorf("%s must be between %d and %d", name, min, max)
	}
	return nil
}

// validateInt64Range validates that an int64 value is within the specified range
func validateInt64Range(value int64, min, max int64, name string) error {
	if value < min || value > max {
		return fmt.Errorf("%s must be between %d and %d", name, min, max)
	}
	return nil
}

// validateFloatRange validates that a float value is within the specified range
func validateFloatRange(value float64, min, max float64, name string) error {
	if value < min || value > max {
		return fmt.Errorf("%s must be between %.1f and %.1f", name, min, max)
	}
	return nil
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
	if err := validateRange(a.fileConfig.MaxTokens, 1, 32768, "max_tokens"); err != nil {
		return err
	}

	if err := validateFloatRange(a.fileConfig.Temperature, 0.0, 2.0, "temperature"); err != nil {
		return err
	}

	if err := validateRange(a.fileConfig.MaxAPICalls, 1, 1000, "max_api_calls"); err != nil {
		return err
	}

	if err := validateRange(a.fileConfig.TimeoutSeconds, 1, 3600, "timeout_seconds"); err != nil {
		return err
	}

	if err := validateInt64Range(a.fileConfig.MaxFileSize, 1, 100*1024*1024, "max_file_size"); err != nil {
		return err
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
	fmt.Fprintf(os.Stderr, "   Spawn Calls:        %d\n", toolStats.SpawnCalls)
	fmt.Fprintf(os.Stderr, "   Exit Calls:         %d\n", toolStats.ExitCalls)
	fmt.Fprintf(os.Stderr, "   Total Tool Calls:   %d\n", toolStats.ReadCalls+toolStats.WriteCalls+toolStats.SpawnCalls+toolStats.ExitCalls)
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
		fmt.Fprintf(os.Stderr, "   Tools/API Call:      %.2f\n", float64(toolStats.ReadCalls+toolStats.WriteCalls+toolStats.SpawnCalls+toolStats.ExitCalls)/float64(openaiStats.RequestCount))

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
