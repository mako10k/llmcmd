package app

import (
	"fmt"
	"log"
	"os"

	"github.com/mako10k/llmcmd/internal/cli"
)

// App represents the main application
type App struct {
	config     *cli.Config
	fileConfig *cli.ConfigFile
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

	// TODO: Phase 2 - Initialize OpenAI client
	// TODO: Phase 3 - Initialize tool execution engine
	// TODO: Phase 4 - Execute LLM interaction
	// TODO: Phase 5 - Handle results and cleanup

	// Placeholder implementation
	fmt.Fprintf(os.Stderr, "llmcmd: Application initialized successfully\n")
	if a.config.Prompt != "" {
		fmt.Fprintf(os.Stderr, "Prompt: %s\n", a.config.Prompt)
	}
	if a.config.Instructions != "" {
		fmt.Fprintf(os.Stderr, "Instructions: %s\n", a.config.Instructions)
	}
	
	return fmt.Errorf("implementation in progress - Phase 1 complete")
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
