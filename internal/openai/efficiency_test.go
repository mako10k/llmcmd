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
	if !strings.Contains(systemMsg, "TOOLS AVAILABLE") {
		t.Error("System message should contain tools available section")
	}

	if !strings.Contains(systemMsg, "read(fd)") {
		t.Error("System message should mention read tool")
	}

	if !strings.Contains(systemMsg, "write(fd, data") {
		t.Error("System message should mention write tool")
	}

	if !strings.Contains(systemMsg, "spawn(cmd") {
		t.Error("System message should mention spawn tool")
	}

	// Check for critical pattern explanation
	if !strings.Contains(systemMsg, "CRITICAL PATTERN FOR COMMAND OUTPUT") {
		t.Error("System message should explain critical pattern for command output")
	}

	// Check for background-only execution explanation
	if !strings.Contains(systemMsg, "background-only") {
		t.Error("System message should mention background-only execution")
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

	// Check for workflow examples with correct spawn pattern
	if !strings.Contains(systemMsg, "spawn({cmd:\"grep\", args:[\"pattern\"]})") {
		t.Error("System message should contain grep workflow example")
	}

	// Check for command output pattern
	if !strings.Contains(systemMsg, "spawn() → write() → read()") {
		t.Error("System message should contain command output pattern")
	}

	if !strings.Contains(systemMsg, "Example workflows") {
		t.Error("System message should contain example workflow")
	}
}
