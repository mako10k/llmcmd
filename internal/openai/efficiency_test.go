package openai

import (
	"strings"
	"testing"
)

func TestCreateInitialMessages_EfficiencyPrompt(t *testing.T) {
	messages := CreateInitialMessages("", "process file efficiently", []string{"test.txt"}, "", false)

	if len(messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(messages))
	}

	systemMsg := messages[0].Content

	// Check for basic tool definitions
	if !strings.Contains(systemMsg, "AVAILABLE TOOLS") {
		t.Error("System message should contain tools available section")
	}

	if !strings.Contains(systemMsg, "read(fd") {
		t.Error("System message should mention read tool")
	}

	if !strings.Contains(systemMsg, "write(fd, data") {
		t.Error("System message should mention write tool")
	}

	if !strings.Contains(systemMsg, "spawn(script") {
		t.Error("System message should mention spawn tool")
	}

	// Check for shell execution environment explanation
	if !strings.Contains(systemMsg, "SHELL EXECUTION ENVIRONMENT") {
		t.Error("System message should explain shell execution environment")
	}

	// Check for background execution explanation
	if !strings.Contains(systemMsg, "run in background") || !strings.Contains(systemMsg, "Returns immediately") {
		t.Error("System message should mention background execution")
	}

	userMsg := messages[1].Content

	// Check for file descriptor mapping
	if !strings.Contains(userMsg, "FILE DESCRIPTOR MAPPING") {
		t.Error("User message should contain file descriptor mapping")
	}
}

func TestCreateInitialMessages_WorkflowExamples(t *testing.T) {
	messages := CreateInitialMessages("", "test", []string{}, "", false)
	systemMsg := messages[0].Content

	// Check for spawn examples with correct pattern
	if !strings.Contains(systemMsg, "spawn(\"grep ERROR | sort\")") {
		t.Error("System message should contain grep workflow example")
	}

	// Check for built-in commands
	if !strings.Contains(systemMsg, "Built-in text processing commands") {
		t.Error("System message should contain built-in commands info")
	}

	// Check for shell syntax support
	if !strings.Contains(systemMsg, "Full shell syntax") {
		t.Error("System message should contain shell syntax info")
	}
}
