package openai

import (
	"strings"
	"testing"
)

func TestCreateInitialMessages_EfficiencyPrompt(t *testing.T) {
	messages := CreateInitialMessages("", "process file efficiently", []string{"test.txt"}, "", false)
	
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}

	systemMsg := messages[0].Content
	
	// Check for efficiency guidelines
	if !strings.Contains(systemMsg, "EFFICIENCY GUIDELINES") {
		t.Error("System message should contain efficiency guidelines")
	}
	
	if !strings.Contains(systemMsg, "minimize API calls") {
		t.Error("System message should mention minimizing API calls")
	}
	
	if !strings.Contains(systemMsg, "pipe(commands=[]") {
		t.Error("System message should show pipeline syntax")
	}
	
	if !strings.Contains(systemMsg, "chain multiple operations") {
		t.Error("System message should mention chaining operations")
	}

	userMsg := messages[1].Content
	
	// Check for efficiency reminders
	if !strings.Contains(userMsg, "pipe() with command chains") {
		t.Error("User message should contain efficiency reminder")
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
