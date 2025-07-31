package llmsh

import (
	"strings"
	"testing"
)

func TestShellBasicCommands(t *testing.T) {
	shell, err := NewShell(nil)
	if err != nil {
		t.Fatalf("Failed to create shell: %v", err)
	}
	
	tests := []struct {
		name        string
		script      string
		expectError bool
	}{
		{
			name:        "simple echo",
			script:      "echo hello",
			expectError: false,
		},
		{
			name:        "echo with pipe",
			script:      "echo hello | cat",
			expectError: false,
		},
		{
			name:        "help command",
			script:      "help",
			expectError: false,
		},
		{
			name:        "help for specific command",
			script:      "help echo",
			expectError: false,
		},
		{
			name:        "conditional execution true",
			script:      "true && echo success",
			expectError: false,
		},
		{
			name:        "conditional execution false",
			script:      "false || echo fallback",
			expectError: false,
		},
		{
			name:        "invalid command",
			script:      "nonexistent_command",
			expectError: true,
		},
	}
	
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := shell.Execute(test.script)
			
			if test.expectError && err == nil {
				t.Errorf("Expected error for script '%s', but got none", test.script)
			}
			
			if !test.expectError && err != nil {
				t.Errorf("Unexpected error for script '%s': %v", test.script, err)
			}
		})
	}
}

func TestShellPipelineExecution(t *testing.T) {
	shell, err := NewShell(nil)
	if err != nil {
		t.Fatalf("Failed to create shell: %v", err)
	}
	
	// Test pipeline with built-in commands
	script := "echo hello world | tr ' ' '\\n'"
	err = shell.Execute(script)
	if err != nil {
		t.Errorf("Pipeline execution failed: %v", err)
	}
}

func TestHelpSystem(t *testing.T) {
	help := NewHelpSystem()
	
	// Test help for existing command
	helpText, err := help.FormatHelp("echo")
	if err != nil {
		t.Errorf("Failed to get help for echo: %v", err)
	}
	
	if !strings.Contains(helpText, "echo") {
		t.Errorf("Help text should contain command name")
	}
	
	// Test help for non-existing command
	_, err = help.FormatHelp("nonexistent")
	if err == nil {
		t.Errorf("Expected error for non-existing command")
	}
	
	// Test command list
	commands := help.ListCommands()
	if len(commands) == 0 {
		t.Errorf("Command list should not be empty")
	}
	
	// Check if basic commands are included
	commandMap := make(map[string]bool)
	for _, cmd := range commands {
		commandMap[cmd] = true
	}
	
	requiredCommands := []string{"echo", "help", "cat", "grep", "llmcmd"}
	for _, cmd := range requiredCommands {
		if !commandMap[cmd] {
			t.Errorf("Command '%s' should be in the list", cmd)
		}
	}
}
