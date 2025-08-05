package app

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestLLMChatProtocolIntegration tests the complete LLM_CHAT protocol flow
func TestLLMChatProtocolIntegration(t *testing.T) {
	// Create mock VFS
	vfs := NewMockVFS(false)

	// Create FSProxy
	proxy := &FSProxyManager{
		vfs:     vfs,
		fdTable: NewFileDescriptorTable(),
	}

	// Simulate LLM_CHAT request with protocol data format
	inputFilesText := "file1.txt\nfile2.txt"
	promptText := "Analyze these files and provide a summary"
	data := []byte(inputFilesText + "\n" + promptText)

	// Test LLM_CHAT handler
	response := proxy.handleLLMChat(true, 0, 1, 2, data)

	// Verify response status
	if response.Status != "OK" {
		t.Errorf("Expected status OK, got %s", response.Status)
	}

	// Parse response data
	lines := strings.SplitN(response.Data, "\n", 2)
	if len(lines) != 2 {
		t.Fatalf("Expected response in format 'size quota\\ndata', got %q", response.Data)
	}

	statusLine := lines[0]
	responseJSON := lines[1]

	// Verify status line format
	parts := strings.Split(statusLine, " ")
	if len(parts) < 2 {
		t.Errorf("Expected status line 'size quota_status', got %q", statusLine)
	}

	// Verify response is valid JSON
	var chatResponse map[string]interface{}
	if err := json.Unmarshal([]byte(responseJSON), &chatResponse); err != nil {
		t.Errorf("Response JSON parsing failed: %v", err)
	}

	// Verify ChatCompletion structure
	if choices, ok := chatResponse["choices"].([]interface{}); !ok || len(choices) == 0 {
		t.Errorf("Response missing choices array")
	}

	if usage, ok := chatResponse["usage"].(map[string]interface{}); !ok {
		t.Errorf("Response missing usage information")
	} else {
		if _, ok := usage["prompt_tokens"]; !ok {
			t.Errorf("Response missing prompt_tokens in usage")
		}
		if _, ok := usage["completion_tokens"]; !ok {
			t.Errorf("Response missing completion_tokens in usage")
		}
	}
}

// TestLLMQuotaProtocolIntegration tests the complete LLM_QUOTA protocol flow
func TestLLMQuotaProtocolIntegration(t *testing.T) {
	// Create mock VFS
	vfs := NewMockVFS(false)

	// Create FSProxy
	proxy := &FSProxyManager{
		vfs:     vfs,
		fdTable: NewFileDescriptorTable(),
	}

	// Test LLM_QUOTA handler
	response := proxy.handleLLMQuota()

	// Verify response status
	if response.Status != "OK" {
		t.Errorf("Expected status OK, got %s", response.Status)
	}

	// Verify quota format
	if !strings.Contains(response.Data, "weighted tokens") {
		t.Errorf("Expected quota information to contain 'weighted tokens', got %q", response.Data)
	}

	// Verify quota data format (should include usage statistics)
	if !strings.Contains(response.Data, "/") {
		t.Errorf("Expected quota format 'used/total', got %q", response.Data)
	}
}

// TestLLMCommandsErrorHandling tests error conditions for LLM commands
func TestLLMCommandsErrorHandling(t *testing.T) {
	// Create mock VFS
	vfs := NewMockVFS(false)

	// Create FSProxy
	proxy := &FSProxyManager{
		vfs:     vfs,
		fdTable: NewFileDescriptorTable(),
	}

	t.Run("LLM_CHAT with invalid FDs", func(t *testing.T) {
		data := []byte("input.txt\nTest prompt")
		response := proxy.handleLLMChat(true, -1, -1, -1, data)

		if response.Status != "ERROR" {
			t.Errorf("Expected ERROR status for invalid FDs, got %s", response.Status)
		}

		if !strings.Contains(response.Data, "invalid file descriptors") {
			t.Errorf("Expected error about invalid FDs, got %q", response.Data)
		}
	})

	t.Run("LLM_CHAT with malformed data", func(t *testing.T) {
		data := []byte("malformed data without newline separator")
		response := proxy.handleLLMChat(true, 0, 1, 2, data)

		if response.Status != "ERROR" {
			t.Errorf("Expected ERROR status for malformed data, got %s", response.Status)
		}

		if !strings.Contains(response.Data, "invalid data format") {
			t.Errorf("Expected error about invalid data format, got %q", response.Data)
		}
	})
}
