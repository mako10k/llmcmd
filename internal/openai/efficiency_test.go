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
	if !strings.Contains(systemMsg, "CORE TOOLS") {
		t.Error("System message should contain core tools section")
	}

	if !strings.Contains(systemMsg, "read(fd)") {
		t.Error("System message should mention read tool")
	}

	if !strings.Contains(systemMsg, "help(keys)") {
		t.Error("System message should mention help tool")
	}

	// Check for workflow pattern
	if !strings.Contains(systemMsg, "WORKFLOW") {
		t.Error("System message should contain workflow section")
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

	// Check for usage help reference
	if !strings.Contains(systemMsg, "help") {
		t.Error("System message should mention help for help")
	}

	// Check for built-in commands
	if !strings.Contains(systemMsg, "Built-in only") {
		t.Error("System message should mention built-in commands")
	}

	// Check for pipe behavior
	if !strings.Contains(systemMsg, "PIPE behavior") {
		t.Error("System message should mention PIPE behavior")
	}
}
