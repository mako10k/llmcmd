package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ConfigFile represents configuration loaded from file
type ConfigFile struct {
	OpenAIAPIKey     string `json:"openai_api_key"`
	OpenAIBaseURL    string `json:"openai_base_url"`
	Model            string `json:"model"`
	MaxTokens        int    `json:"max_tokens"`
	Temperature      float64 `json:"temperature"`
	MaxAPICalls      int    `json:"max_api_calls"`
	TimeoutSeconds   int    `json:"timeout_seconds"`
	MaxFileSize      int64  `json:"max_file_size"`
	ReadBufferSize   int    `json:"read_buffer_size"`
}

// DefaultConfig returns default configuration values
func DefaultConfig() *ConfigFile {
	return &ConfigFile{
		OpenAIBaseURL:    "https://api.openai.com/v1",
		Model:            "gpt-4o-mini",
		MaxTokens:        4096,
		Temperature:      0.1,
		MaxAPICalls:      50,
		TimeoutSeconds:   300,
		MaxFileSize:      10 * 1024 * 1024, // 10MB
		ReadBufferSize:   4096,             // 4KB
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
		if val, err := parseInt(value); err != nil {
			return fmt.Errorf("invalid max_tokens: %w", err)
		} else {
			config.MaxTokens = val
		}
	case "temperature":
		if val, err := parseFloat(value); err != nil {
			return fmt.Errorf("invalid temperature: %w", err)
		} else {
			config.Temperature = val
		}
	case "max_api_calls":
		if val, err := parseInt(value); err != nil {
			return fmt.Errorf("invalid max_api_calls: %w", err)
		} else {
			config.MaxAPICalls = val
		}
	case "timeout_seconds":
		if val, err := parseInt(value); err != nil {
			return fmt.Errorf("invalid timeout_seconds: %w", err)
		} else {
			config.TimeoutSeconds = val
		}
	case "max_file_size":
		if val, err := parseInt64(value); err != nil {
			return fmt.Errorf("invalid max_file_size: %w", err)
		} else {
			config.MaxFileSize = val
		}
	case "read_buffer_size":
		if val, err := parseInt(value); err != nil {
			return fmt.Errorf("invalid read_buffer_size: %w", err)
		} else {
			config.ReadBufferSize = val
		}
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
