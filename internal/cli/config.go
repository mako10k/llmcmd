package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PromptPreset represents a predefined prompt configuration
type PromptPreset struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

// ConfigFile represents configuration loaded from file
type ConfigFile struct {
	OpenAIAPIKey    string                   `json:"openai_api_key"`
	OpenAIBaseURL   string                   `json:"openai_base_url"`
	Model           string                   `json:"model"`
	MaxTokens       int                      `json:"max_tokens"`
	Temperature     float64                  `json:"temperature"`
	MaxAPICalls     int                      `json:"max_api_calls"`
	TimeoutSeconds  int                      `json:"timeout_seconds"`
	MaxFileSize     int64                    `json:"max_file_size"`
	ReadBufferSize  int                      `json:"read_buffer_size"`
	MaxRetries      int                      `json:"max_retries"`
	RetryDelay      int                      `json:"retry_delay_ms"`
	SystemPrompt    string                   `json:"system_prompt"`
	DefaultPrompt   string                   `json:"default_prompt"`
	DisableTools    bool                     `json:"disable_tools"`
	PromptPresets   map[string]PromptPreset  `json:"prompt_presets"`
}

// DefaultConfig returns default configuration values
func DefaultConfig() *ConfigFile {
	return &ConfigFile{
		OpenAIBaseURL:  "https://api.openai.com/v1",
		Model:          "gpt-4o-mini",
		MaxTokens:      4096,
		Temperature:    0.1,
		MaxAPICalls:    50,
		TimeoutSeconds: 300,
		MaxFileSize:    10 * 1024 * 1024, // 10MB
		ReadBufferSize: 4096,             // 4KB
		MaxRetries:     3,
		RetryDelay:     1000,  // 1 second
		SystemPrompt:   "",    // Empty means use default built-in prompt
		DefaultPrompt:  "general", // Default preset key
		DisableTools:   false, // Tools enabled by default
		PromptPresets:  getDefaultPromptPresets(),
	}
}

// LoadConfigFile loads configuration from file
func LoadConfigFile(path string) (*ConfigFile, error) {
	config := DefaultConfig()

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// File doesn't exist, return default config
		return config, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse key=value pairs
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid config line %d: %s", lineNum, line)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}

		// Set configuration values
		if err := setConfigValue(config, key, value); err != nil {
			return nil, fmt.Errorf("config line %d: %w", lineNum, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	return config, nil
}

// ResolvePreset resolves a preset key to its content from the configuration
func ResolvePreset(config *ConfigFile, presetKey string) (string, error) {
	if config == nil {
		return "", fmt.Errorf("configuration is nil")
	}
	
	if config.PromptPresets == nil {
		return "", fmt.Errorf("no presets available in configuration")
	}
	
	preset, exists := config.PromptPresets[presetKey]
	if !exists {
		return "", fmt.Errorf("preset '%s' not found", presetKey)
	}
	
	return preset.Content, nil
}

// LoadAndMergeConfig loads configuration from file and merges with CLI arguments
func LoadAndMergeConfig(cliConfig *Config) (*ConfigFile, error) {
	// Start with default configuration
	config := DefaultConfig()
	
	// Load from config file if specified
	configFile := cliConfig.ConfigFile
	if configFile == "" {
		// Use default config file path
		homeDir, err := os.UserHomeDir()
		if err == nil {
			configFile = filepath.Join(homeDir, ".llmcmdrc")
		}
	}
	
	if configFile != "" {
		fileConfig, err := LoadConfigFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
		
		// Merge file config with defaults
		if fileConfig.OpenAIAPIKey != "" {
			config.OpenAIAPIKey = fileConfig.OpenAIAPIKey
		}
		if fileConfig.OpenAIBaseURL != "" {
			config.OpenAIBaseURL = fileConfig.OpenAIBaseURL
		}
		if fileConfig.Model != "" {
			config.Model = fileConfig.Model
		}
		if fileConfig.MaxTokens != 0 {
			config.MaxTokens = fileConfig.MaxTokens
		}
		if fileConfig.Temperature != 0 {
			config.Temperature = fileConfig.Temperature
		}
		if fileConfig.MaxAPICalls != 0 {
			config.MaxAPICalls = fileConfig.MaxAPICalls
		}
		if fileConfig.TimeoutSeconds != 0 {
			config.TimeoutSeconds = fileConfig.TimeoutSeconds
		}
		if fileConfig.MaxFileSize != 0 {
			config.MaxFileSize = fileConfig.MaxFileSize
		}
		if fileConfig.ReadBufferSize != 0 {
			config.ReadBufferSize = fileConfig.ReadBufferSize
		}
		if fileConfig.MaxRetries != 0 {
			config.MaxRetries = fileConfig.MaxRetries
		}
		if fileConfig.RetryDelay != 0 {
			config.RetryDelay = fileConfig.RetryDelay
		}
		if fileConfig.SystemPrompt != "" {
			config.SystemPrompt = fileConfig.SystemPrompt
		}
		if fileConfig.DefaultPrompt != "" {
			config.DefaultPrompt = fileConfig.DefaultPrompt
		}
		config.DisableTools = fileConfig.DisableTools
		
		// Merge presets (file presets override defaults)
		if fileConfig.PromptPresets != nil {
			for k, v := range fileConfig.PromptPresets {
				config.PromptPresets[k] = v
			}
		}
	}
	
	// Apply CLI overrides
	// TODO: Apply CLI configuration overrides here
	
	return config, nil
}

// getDefaultPromptPresets returns the default prompt presets
func getDefaultPromptPresets() map[string]PromptPreset {
	return map[string]PromptPreset{
		"general": {
			Key:         "general",
			Description: "General-purpose prompt for various tasks",
			Content: `You are an intelligent assistant that can execute text processing tasks using built-in commands.
Process the user's request step by step and provide clear, helpful responses.
When using tools, explain what you're doing and show the results.`,
		},
		"diff_patch": {
			Key:         "diff_patch",
			Description: "Specialized prompt for diff/patch operations and file comparison",
			Content: `You are a specialized tool for diff and patch operations. Your role is to:

1. ALWAYS execute diff/patch commands when requested
2. REPORT EXACTLY what the tools output - do not interpret or modify the results
3. If tools output nothing, report "No output from command" - do not assume "no differences"
4. For diff operations: show the actual diff output, line by line
5. For patch operations: report success/failure and any error messages
6. When comparing files: use the exact output from built-in diff commands

Execute the requested diff/patch operations and provide the raw tool output without interpretation.`,
		},
		"code_review": {
			Key:         "code_review",
			Description: "Focused prompt for code analysis and review tasks",
			Content: `You are a code analysis specialist. Your role is to:

1. Analyze code structure, patterns, and quality
2. Identify potential issues, bugs, or improvements
3. Suggest best practices and optimizations
4. Explain complex code sections clearly
5. Use built-in tools (grep, diff) to examine code thoroughly

Provide detailed, constructive feedback with specific examples and suggestions.`,
		},
		"data_proc": {
			Key:         "data_proc",
			Description: "Optimized prompt for data processing and text manipulation",
			Content: `You are a data processing specialist. Your role is to:

1. Efficiently process large text files and data streams
2. Use built-in commands (grep, sed, sort, wc, tr) for text manipulation
3. Transform and filter data according to user requirements
4. Provide clear summaries of data processing operations
5. Handle structured and unstructured text data

Focus on accurate, efficient data processing with clear explanations of transformations.`,
		},
	}
}

// setConfigValue sets a configuration value by key
func setConfigValue(config *ConfigFile, key, value string) error {
	switch key {
	case "openai_api_key":
		config.OpenAIAPIKey = value
	case "openai_base_url":
		config.OpenAIBaseURL = value
	case "model":
		config.Model = value
	case "max_tokens":
		return parseAndAssignInt(value, "max_tokens", func(val int) { config.MaxTokens = val })
	case "temperature":
		return parseAndAssignFloat(value, "temperature", func(val float64) { config.Temperature = val })
	case "max_api_calls":
		return parseAndAssignInt(value, "max_api_calls", func(val int) { config.MaxAPICalls = val })
	case "timeout_seconds":
		return parseAndAssignInt(value, "timeout_seconds", func(val int) { config.TimeoutSeconds = val })
	case "max_file_size":
		return parseAndAssignInt64(value, "max_file_size", func(val int64) { config.MaxFileSize = val })
	case "read_buffer_size":
		return parseAndAssignInt(value, "read_buffer_size", func(val int) { config.ReadBufferSize = val })
	case "max_retries":
		return parseAndAssignInt(value, "max_retries", func(val int) { config.MaxRetries = val })
	case "retry_delay_ms":
		return parseAndAssignInt(value, "retry_delay_ms", func(val int) { config.RetryDelay = val })
	case "system_prompt":
		config.SystemPrompt = value
	case "disable_tools":
		return parseAndAssignBool(value, "disable_tools", func(val bool) { config.DisableTools = val })
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}
	return nil
}

// Helper functions for parsing
func parseInt(s string) (int, error) {
	var val int
	_, err := fmt.Sscanf(s, "%d", &val)
	return val, err
}

func parseInt64(s string) (int64, error) {
	var val int64
	_, err := fmt.Sscanf(s, "%d", &val)
	return val, err
}

func parseFloat(s string) (float64, error) {
	var val float64
	_, err := fmt.Sscanf(s, "%f", &val)
	return val, err
}

func parseBool(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "true", "1", "yes", "on", "enable", "enabled":
		return true, nil
	case "false", "0", "no", "off", "disable", "disabled":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value: %s", s)
	}
}

// parseAndAssignInt parses an integer value and assigns it via a setter function
func parseAndAssignInt(value string, fieldName string, setter func(int)) error {
	if val, err := parseInt(value); err != nil {
		return fmt.Errorf("invalid %s: %w", fieldName, err)
	} else {
		setter(val)
	}
	return nil
}

// parseAndAssignInt64 parses an int64 value and assigns it via a setter function
func parseAndAssignInt64(value string, fieldName string, setter func(int64)) error {
	if val, err := parseInt64(value); err != nil {
		return fmt.Errorf("invalid %s: %w", fieldName, err)
	} else {
		setter(val)
	}
	return nil
}

// parseAndAssignFloat parses a float value and assigns it via a setter function
func parseAndAssignFloat(value string, fieldName string, setter func(float64)) error {
	if val, err := parseFloat(value); err != nil {
		return fmt.Errorf("invalid %s: %w", fieldName, err)
	} else {
		setter(val)
	}
	return nil
}

// parseAndAssignBool parses a boolean value and assigns it via a setter function
func parseAndAssignBool(value string, fieldName string, setter func(bool)) error {
	if val, err := parseBool(value); err != nil {
		return fmt.Errorf("invalid %s: %w", fieldName, err)
	} else {
		setter(val)
	}
	return nil
}

// LoadEnvironmentConfig loads configuration from environment variables
func LoadEnvironmentConfig(config *ConfigFile) {
	if val := os.Getenv("OPENAI_API_KEY"); val != "" {
		config.OpenAIAPIKey = val
	}
	if val := os.Getenv("OPENAI_BASE_URL"); val != "" {
		config.OpenAIBaseURL = val
	}
	if val := os.Getenv("LLMCMD_MODEL"); val != "" {
		config.Model = val
	}
	if val := os.Getenv("LLMCMD_MAX_TOKENS"); val != "" {
		if parsed, err := parseInt(val); err == nil {
			config.MaxTokens = parsed
		}
	}
	if val := os.Getenv("LLMCMD_TEMPERATURE"); val != "" {
		if parsed, err := parseFloat(val); err == nil {
			config.Temperature = parsed
		}
	}
	if val := os.Getenv("LLMCMD_MAX_API_CALLS"); val != "" {
		if parsed, err := parseInt(val); err == nil {
			config.MaxAPICalls = parsed
		}
	}
	if val := os.Getenv("LLMCMD_TIMEOUT_SECONDS"); val != "" {
		if parsed, err := parseInt(val); err == nil {
			config.TimeoutSeconds = parsed
		}
	}
}
