package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
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

// QuotaWeights represents cost weights for different token types
type QuotaWeights struct {
	InputWeight       float64 `json:"input_weight"`        // Weight for input tokens
	InputCachedWeight float64 `json:"input_cached_weight"` // Weight for cached input tokens
	OutputWeight      float64 `json:"output_weight"`       // Weight for output tokens
}

// ModelQuotaWeights defines quota weights for specific models
type ModelQuotaWeights struct {
	Model        string       `json:"model"`
	QuotaWeights QuotaWeights `json:"weights"`
	Description  string       `json:"description"`
}

// ModelSystemPrompt defines model-specific system prompts
type ModelSystemPrompt struct {
	Model       string `json:"model"`
	SystemPrompt string `json:"system_prompt"`
	Description string `json:"description"`
}

// QuotaUsage tracks quota consumption statistics
type QuotaUsage struct {
	TotalWeightedTokens float64 `json:"total_weighted_tokens"` // Total weighted token consumption
	InputTokens         int     `json:"input_tokens"`          // Raw input tokens used
	InputCachedTokens   int     `json:"input_cached_tokens"`   // Raw cached input tokens used
	OutputTokens        int     `json:"output_tokens"`         // Raw output tokens used
	APICalls            int     `json:"api_calls"`             // Number of API calls made
}

// ConfigFile represents configuration loaded from file
type ConfigFile struct {
	OpenAIAPIKey   string                  `json:"openai_api_key"`
	OpenAIBaseURL  string                  `json:"openai_base_url"`
	Model          string                  `json:"model"`            // Primary model for external llmcmd calls
	InternalModel  string                  `json:"internal_model"`  // Model for internal llmcmd calls from llmsh
	MaxTokens      int                     `json:"max_tokens"`
	Temperature    float64                 `json:"temperature"`
	MaxAPICalls    int                     `json:"max_api_calls"`
	TimeoutSeconds int                     `json:"timeout_seconds"`
	MaxFileSize    int64                   `json:"max_file_size"`
	ReadBufferSize int                     `json:"read_buffer_size"`
	MaxRetries     int                     `json:"max_retries"`
	RetryDelay     int                     `json:"retry_delay_ms"`
	SystemPrompt   string                  `json:"system_prompt"`
	DefaultPrompt  string                  `json:"default_prompt"`
	DisableTools   bool                    `json:"disable_tools"`
	PromptPresets  map[string]PromptPreset `json:"prompt_presets"`
	// Quota system configuration
	QuotaMaxTokens    int                     `json:"quota_max_tokens"`    // Maximum weighted tokens allowed
	QuotaWeights      QuotaWeights            `json:"quota_weights"`       // Token type weights
	QuotaUsage        QuotaUsage              `json:"quota_usage"`         // Current usage statistics
	ModelQuotaWeights map[string]QuotaWeights `json:"model_quota_weights"` // Model-specific quota weights
	ModelSystemPrompts map[string]string      `json:"model_system_prompts"` // Model-specific system prompts
}

// DefaultConfig returns default configuration values
func DefaultConfig() *ConfigFile {
	return &ConfigFile{
		OpenAIBaseURL:  "https://api.openai.com/v1",
		Model:          "gpt-4o-mini",
		InternalModel:  "gpt-4o-mini", // Default to same model for internal calls
		MaxTokens:      4096,
		Temperature:    0.1,
		MaxAPICalls:    50,
		TimeoutSeconds: 300,
		MaxFileSize:    10 * 1024 * 1024, // 10MB
		ReadBufferSize: 4096,             // 4KB
		MaxRetries:     3,
		RetryDelay:     1000,      // 1 second
		SystemPrompt:   "",        // Empty means use default built-in prompt
		DefaultPrompt:  "general", // Default preset key
		DisableTools:   false,     // Tools enabled by default
		PromptPresets:  getDefaultPromptPresets(),
		// Default quota configuration (0 means no limit)
		QuotaMaxTokens: 0, // No limit by default
		QuotaWeights: QuotaWeights{
			InputWeight:       1.0,  // Standard input token weight
			InputCachedWeight: 0.25, // Cached tokens cost 25% of input tokens
			OutputWeight:      4.0,  // Output tokens cost 4x input tokens (typical for gpt-4o)
		},
		QuotaUsage: QuotaUsage{
			TotalWeightedTokens: 0,
			InputTokens:         0,
			InputCachedTokens:   0,
			OutputTokens:        0,
			APICalls:            0,
		},
		ModelQuotaWeights:  getDefaultModelQuotaWeights(),
		ModelSystemPrompts: getDefaultModelSystemPrompts(),
	}
}

// LoadConfigFile loads configuration from file
func LoadConfigFile(path string, explicit bool) (*ConfigFile, error) {
	config := DefaultConfig()

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if explicit {
			// File was explicitly specified but doesn't exist - FAIL IMMEDIATELY
			return nil, fmt.Errorf("explicitly specified config file does not exist: %s", path)
		}
		// File was not explicitly specified (default), return default config
		return config, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	// Read first few bytes to detect file format
	buffer := make([]byte, 10)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Reset file pointer
	file.Seek(0, 0)

	// Check if it's JSON format
	firstChar := strings.TrimSpace(string(buffer[:n]))
	if strings.HasPrefix(firstChar, "{") {
		// JSON format - use strict JSON parser
		return loadJSONConfig(file, config)
	}

	// Legacy key=value format
	return loadLegacyConfig(file, config)
}

// loadJSONConfig loads configuration from JSON format with strict error checking
func loadJSONConfig(file *os.File, config *ConfigFile) (*ConfigFile, error) {
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields() // Strict: fail on unknown fields

	if err := decoder.Decode(config); err != nil {
		// JSON parsing failed - FAIL IMMEDIATELY
		return nil, fmt.Errorf("JSON config file parsing failed: %w", err)
	}

	// Validate configuration values after JSON loading
	if err := validateConfigValues(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

// loadLegacyConfig loads configuration from legacy key=value format
func loadLegacyConfig(file *os.File, config *ConfigFile) (*ConfigFile, error) {

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

	// Validate configuration values after legacy loading
	if err := validateConfigValues(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

// validateConfigValues performs strict validation of configuration values
func validateConfigValues(config *ConfigFile) error {
	// Critical validation - any failure should terminate the program

	// Model validation
	if config.Model == "" {
		return fmt.Errorf("model name cannot be empty")
	}

	// Numeric range validation
	if config.MaxTokens < 1 || config.MaxTokens > 32768 {
		return fmt.Errorf("max_tokens must be between 1 and 32768, got %d", config.MaxTokens)
	}

	if config.Temperature < 0.0 || config.Temperature > 2.0 {
		return fmt.Errorf("temperature must be between 0.0 and 2.0, got %.2f", config.Temperature)
	}

	if config.MaxAPICalls < 1 || config.MaxAPICalls > 1000 {
		return fmt.Errorf("max_api_calls must be between 1 and 1000, got %d", config.MaxAPICalls)
	}

	if config.TimeoutSeconds < 1 || config.TimeoutSeconds > 3600 {
		return fmt.Errorf("timeout_seconds must be between 1 and 3600, got %d", config.TimeoutSeconds)
	}

	if config.MaxFileSize < 1 || config.MaxFileSize > 100*1024*1024 {
		return fmt.Errorf("max_file_size must be between 1 and 100MB, got %d", config.MaxFileSize)
	}

	if config.ReadBufferSize < 1 || config.ReadBufferSize > 64*1024 {
		return fmt.Errorf("read_buffer_size must be between 1 and 64KB, got %d", config.ReadBufferSize)
	}

	if config.MaxRetries < 0 || config.MaxRetries > 10 {
		return fmt.Errorf("max_retries must be between 0 and 10, got %d", config.MaxRetries)
	}

	if config.RetryDelay < 0 || config.RetryDelay > 60000 {
		return fmt.Errorf("retry_delay_ms must be between 0 and 60000, got %d", config.RetryDelay)
	}

	// Quota validation
	if config.QuotaMaxTokens < 0 {
		return fmt.Errorf("quota_max_tokens cannot be negative, got %d", config.QuotaMaxTokens)
	}

	if config.QuotaWeights.InputWeight < 0 {
		return fmt.Errorf("quota input_weight cannot be negative, got %.2f", config.QuotaWeights.InputWeight)
	}

	if config.QuotaWeights.InputCachedWeight < 0 {
		return fmt.Errorf("quota input_cached_weight cannot be negative, got %.2f", config.QuotaWeights.InputCachedWeight)
	}

	if config.QuotaWeights.OutputWeight < 0 {
		return fmt.Errorf("quota output_weight cannot be negative, got %.2f", config.QuotaWeights.OutputWeight)
	}

	return nil
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

	if configFile != "" {
		fileConfig, err := LoadConfigFile(configFile, cliConfig.ConfigExplicit)
		if err != nil {
			// Only fail if config was explicitly specified or if it's a parsing error
			if cliConfig.ConfigExplicit || !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to load config file %s: %w", configFile, err)
			}
			// Default config file not found is acceptable - use defaults
		} else {
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
			if fileConfig.MaxTokens > 0 {
				config.MaxTokens = fileConfig.MaxTokens
			}
			if fileConfig.Temperature >= 0 {
				config.Temperature = fileConfig.Temperature
			}
			if fileConfig.MaxAPICalls > 0 {
				config.MaxAPICalls = fileConfig.MaxAPICalls
			}
			if fileConfig.TimeoutSeconds > 0 {
				config.TimeoutSeconds = fileConfig.TimeoutSeconds
			}
			if fileConfig.MaxFileSize > 0 {
				config.MaxFileSize = fileConfig.MaxFileSize
			}
			if fileConfig.ReadBufferSize > 0 {
				config.ReadBufferSize = fileConfig.ReadBufferSize
			}
			if fileConfig.MaxRetries > 0 {
				config.MaxRetries = fileConfig.MaxRetries
			}
			if fileConfig.RetryDelay > 0 {
				config.RetryDelay = fileConfig.RetryDelay
			}
			if fileConfig.SystemPrompt != "" {
				config.SystemPrompt = fileConfig.SystemPrompt
			}
			if fileConfig.DefaultPrompt != "" {
				config.DefaultPrompt = fileConfig.DefaultPrompt
			}
			config.DisableTools = fileConfig.DisableTools

			// Merge quota configuration
			if fileConfig.QuotaMaxTokens > 0 {
				config.QuotaMaxTokens = fileConfig.QuotaMaxTokens
			}
			if fileConfig.QuotaWeights.InputWeight > 0 {
				config.QuotaWeights = fileConfig.QuotaWeights
			}

			// Merge presets (file presets override defaults)
			if fileConfig.PromptPresets != nil {
				for k, v := range fileConfig.PromptPresets {
					config.PromptPresets[k] = v
				}
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

// UpdateQuotaUsage updates quota usage statistics
func (c *ConfigFile) UpdateQuotaUsage(inputTokens, cachedTokens, outputTokens int) {
	// Update raw token counts
	c.QuotaUsage.InputTokens += inputTokens
	c.QuotaUsage.InputCachedTokens += cachedTokens
	c.QuotaUsage.OutputTokens += outputTokens
	c.QuotaUsage.APICalls++

	// Calculate weighted usage using effective model weights
	effectiveWeights := c.GetEffectiveQuotaWeights()
	weightedInput := float64(inputTokens) * effectiveWeights.InputWeight
	weightedCached := float64(cachedTokens) * effectiveWeights.InputCachedWeight
	weightedOutput := float64(outputTokens) * effectiveWeights.OutputWeight

	c.QuotaUsage.TotalWeightedTokens += weightedInput + weightedCached + weightedOutput
}

// IsQuotaExceeded checks if quota limit has been exceeded
func (c *ConfigFile) IsQuotaExceeded() bool {
	if c.QuotaMaxTokens <= 0 {
		return false // No limit set
	}
	return c.QuotaUsage.TotalWeightedTokens >= float64(c.QuotaMaxTokens)
}

// GetQuotaStatusString returns a formatted quota status string for system prompts
func (c *ConfigFile) GetQuotaStatusString() string {
	// API calls information
	remainingCalls := c.MaxAPICalls - c.QuotaUsage.APICalls
	var apiStatus string
	if remainingCalls > 0 {
		apiStatus = fmt.Sprintf("API Calls: %d/%d (remaining: %d)",
			c.QuotaUsage.APICalls, c.MaxAPICalls, remainingCalls)
	} else {
		apiStatus = fmt.Sprintf("API Calls: %d/%d (LIMIT REACHED)",
			c.QuotaUsage.APICalls, c.MaxAPICalls)
	}

	// Token usage information
	var tokenStatus string
	if c.QuotaMaxTokens <= 0 {
		tokenStatus = fmt.Sprintf("Token Usage: %.1f weighted tokens (no limit)",
			c.QuotaUsage.TotalWeightedTokens)
	} else {
		remaining := float64(c.QuotaMaxTokens) - c.QuotaUsage.TotalWeightedTokens
		percentage := (c.QuotaUsage.TotalWeightedTokens / float64(c.QuotaMaxTokens)) * 100
		tokenStatus = fmt.Sprintf("Token Usage: %.1f/%.0f weighted tokens (%.1f%% used, %.1f remaining)",
			c.QuotaUsage.TotalWeightedTokens, float64(c.QuotaMaxTokens), percentage, remaining)
	}

	return fmt.Sprintf("%s\n%s", apiStatus, tokenStatus)
}

// GetEffectiveQuotaWeights returns the quota weights for the current model
func (c *ConfigFile) GetEffectiveQuotaWeights() QuotaWeights {
	// Initialize ModelQuotaWeights if it's empty (for backward compatibility)
	if c.ModelQuotaWeights == nil {
		c.ModelQuotaWeights = getDefaultModelQuotaWeights()
	}
	
	// Check if model-specific weights exist
	if modelWeights, exists := c.ModelQuotaWeights[c.Model]; exists {
		return modelWeights
	}
	
	// Fall back to default weights
	return c.QuotaWeights
}

// GetEffectiveSystemPrompt returns the system prompt for the current model
func (c *ConfigFile) GetEffectiveSystemPrompt() string {
	// If user has set a custom system prompt, use it regardless of model
	if c.SystemPrompt != "" {
		return c.SystemPrompt
	}
	
	// Initialize ModelSystemPrompts if it's empty (for backward compatibility)
	if c.ModelSystemPrompts == nil {
		c.ModelSystemPrompts = getDefaultModelSystemPrompts()
	}
	
	// Check if model-specific system prompt exists
	if modelPrompt, exists := c.ModelSystemPrompts[c.Model]; exists {
		return modelPrompt
	}
	
	// Fall back to empty string (will use default built-in prompt)
	return ""
} // SaveConfigFile saves the current configuration to file
func (c *ConfigFile) SaveConfigFile(path string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal config to JSON
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// getDefaultModelQuotaWeights returns default model-specific quota weights
func getDefaultModelQuotaWeights() map[string]QuotaWeights {
	return map[string]QuotaWeights{
		"gpt-4o-mini": {
			InputWeight:       1.0,   // Base model: $0.150 / 1M tokens
			InputCachedWeight: 0.25,  // 50% discount: $0.075 / 1M tokens  
			OutputWeight:      4.0,   // $0.600 / 1M tokens
		},
		"gpt-4o": {
			InputWeight:       16.67, // $2.50 / 1M tokens (16.67x of gpt-4o-mini)
			InputCachedWeight: 8.33,  // $1.25 / 1M tokens (50% discount)
			OutputWeight:      66.67, // $10.00 / 1M tokens (16.67x of gpt-4o-mini)
		},
		"o1-mini": {
			InputWeight:       20.0,  // $3.00 / 1M tokens (20x of gpt-4o-mini)
			InputCachedWeight: 20.0,  // No caching discount for o1 models
			OutputWeight:      80.0,  // $12.00 / 1M tokens (20x of gpt-4o-mini)
		},
		"o1-preview": {
			InputWeight:       100.0, // $15.00 / 1M tokens (100x of gpt-4o-mini)
			InputCachedWeight: 100.0, // No caching discount for o1 models
			OutputWeight:      400.0, // $60.00 / 1M tokens (100x of gpt-4o-mini)
		},
		"o3-mini": {
			InputWeight:       20.0,  // Estimated same as o1-mini
			InputCachedWeight: 20.0,  // No caching discount for o3 models
			OutputWeight:      80.0,  // Estimated same as o1-mini
		},
	}
}

// getDefaultModelSystemPrompts returns default model-specific system prompts
func getDefaultModelSystemPrompts() map[string]string {
	return map[string]string{
		"o1-mini": `You are llmcmd, the intelligent text processing core of llmsh. You excel at complex reasoning and multi-step analysis through secure tool interfaces.

🏠 LLMSH INTEGRATION: llmsh is an LLM-powered shell that provides intelligent command execution. You serve as the text processing engine for complex data tasks.

🧠 O1-MINI REASONING:
- Apply step-by-step logical thinking to text processing challenges
- Break down complex problems systematically  
- Plan efficient multi-tool workflows
- Handle ambiguous requirements through reasoning

🔧 TOOLS: read, write, open, spawn, close, exit
🛠️ SHELL ACCESS: Full shell environment via spawn(script)
⚠️ SECURITY: Secure sandboxed execution environment

🎯 WORKFLOW: Leverage reasoning capabilities for optimal text processing solutions`,

		"o1-preview": `You are llmcmd, the advanced reasoning core of llmsh. You excel at sophisticated problem-solving and complex data analysis through intelligent tool usage.

🏠 LLMSH INTEGRATION: llmsh is an LLM-powered shell environment. You provide the advanced reasoning engine for complex text processing and data manipulation tasks.

🧠 O1-PREVIEW EXCELLENCE:
- Apply sophisticated logical analysis to complex text challenges
- Design optimal multi-stage data processing pipelines
- Anticipate edge cases and provide robust solutions
- Handle highly complex and ambiguous requirements

🔧 TOOLS: read, write, open, spawn, close, exit  
🛠️ SHELL ACCESS: Complete shell environment via spawn(script)
⚠️ SECURITY: Advanced sandboxed execution with full capability

🎯 APPROACH: Use superior reasoning for complex text processing workflows`,

		"gpt-4o": `You are llmcmd, the capable text processing engine within llmsh. You handle complex tasks with excellent context understanding and efficient execution.

🏠 LLMSH INTEGRATION: llmsh is an intelligent shell environment. You provide sophisticated text processing capabilities with strong contextual awareness.

🎯 GPT-4O STRENGTHS:
- Superior context understanding for complex requests
- Efficient multi-file and data format handling
- Strong pattern recognition and analysis
- Optimized workflow execution

� TOOLS: read, write, open, spawn, close, exit
🛠️ SHELL ACCESS: Full command environment via spawn(script)  
⚠️ SECURITY: Secure tool-based execution environment

🎯 FOCUS: Deliver efficient, accurate text processing with contextual intelligence`,
	}
}
