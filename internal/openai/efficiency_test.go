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

	if !strings.Contains(systemMsg, "write(fd, data)") {
		t.Error("System message should mention write tool")
	}

	if !strings.Contains(systemMsg, "pipe(commands, input)") {
		t.Error("System message should mention pipe tool")
	}

	userMsg := messages[1].Content

	// Check for file descriptor mapping
	if !strings.Contains(userMsg, "FILE DESCRIPTOR MAPPING") {
		t.Error("User message should contain file descriptor mapping")
	}
}

func TestCreateInitialMessages_PipelineExample(t *testing.T) {
	messages := CreateInitialMessages("", "test", []string{}, "", false)
	systemMsg := messages[0].Content

	// Check for pipeline example
	if !strings.Contains(systemMsg, `[{name:"grep",args:["apple"]},{name:"sort",args:["-u"]}]`) {
		t.Error("System message should contain pipeline example")
	}

	if !strings.Contains(systemMsg, "EXAMPLE WORKFLOW") {
		t.Error("System message should contain example workflow")
	}
}
