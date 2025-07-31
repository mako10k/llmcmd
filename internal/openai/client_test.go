package openai

import (
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	config := ClientConfig{
		APIKey:   "test-key",
		BaseURL:  "https://api.openai.com/v1",
		Timeout:  30 * time.Second,
		MaxCalls: 10,
	}

	client := NewClient(config)
	if client == nil {
		t.Fatal("NewClient returned nil")
	}

	stats := client.GetStats()
	if stats.RequestCount != 0 {
		t.Errorf("Initial request count should be 0, got %d", stats.RequestCount)
	}
}

func TestToolDefinitions(t *testing.T) {
	tools := ToolDefinitions()
	if len(tools) != 6 {
		t.Errorf("Expected 6 tools, got %d", len(tools))
	}

	expected := map[string]bool{
		"read":  false,
		"write": false,
		"open":  false,
		"spawn": false,
		"close": false,
		"exit":  false,
	}

	for _, tool := range tools {
		if _, exists := expected[tool.Function.Name]; exists {
			expected[tool.Function.Name] = true
		} else {
			t.Errorf("Unexpected tool: %s", tool.Function.Name)
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("Missing tool: %s", name)
		}
	}
}

func TestCreateInitialMessages(t *testing.T) {
	messages := CreateInitialMessages("test prompt", "test instruction", []string{"file1.txt"}, "", false)

	if len(messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(messages))
	}

	if messages[0].Role != "system" {
		t.Errorf("First message should be system role, got %s", messages[0].Role)
	}

	if messages[1].Role != "user" {
		t.Errorf("Second message should be user role, got %s", messages[1].Role)
	}

	if messages[2].Role != "user" {
		t.Errorf("Third message should be user role, got %s", messages[2].Role)
	}
}
