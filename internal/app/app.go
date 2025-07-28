package app

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mako10k/llmcmd/internal/cli"
	"github.com/mako10k/llmcmd/internal/openai"
	"github.com/mako10k/llmcmd/internal/tools"
)

// App represents the main application
type App struct {
	config     *cli.Config
	fileConfig *cli.ConfigFile
	openaiClient *openai.Client
	toolEngine   *tools.Engine
}

// New creates a new application instance
func New(config *cli.Config) *App {
	return &App{
		config: config,
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

	return nil
}

// initializeOpenAI initializes the OpenAI client
func (a *App) initializeOpenAI() error {
	config := openai.ClientConfig{
		APIKey:  a.fileConfig.OpenAIAPIKey,
		BaseURL: a.fileConfig.OpenAIBaseURL,
		Timeout: time.Duration(a.fileConfig.TimeoutSeconds) * time.Second,
		MaxCalls: a.fileConfig.MaxAPICalls,
	}

	a.openaiClient = openai.NewClient(config)

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
		// Create request
		request := openai.ChatCompletionRequest{
			Model:       a.fileConfig.Model,
			Messages:    messages,
			Tools:       openai.ToolDefinitions(),
			ToolChoice:  "auto",
			MaxTokens:   a.fileConfig.MaxTokens,
			Temperature: a.fileConfig.Temperature,
		}

		// Send request to OpenAI
		response, err := a.openaiClient.ChatCompletion(ctx, request)
		if err != nil {
			return fmt.Errorf("OpenAI API error: %w", err)
		}

		// Process response
		choice := response.Choices[0]
		messages = append(messages, choice.Message)

		if a.config.Verbose {
			stats := a.openaiClient.GetStats()
			log.Printf("API call completed (total: %d/%d, tokens: %d)", 
				stats.RequestCount, a.fileConfig.MaxAPICalls, response.Usage.TotalTokens)
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
