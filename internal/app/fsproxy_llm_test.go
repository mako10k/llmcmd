package app

import (
	"strings"
	"testing"
)

func TestLLMChatHandler(t *testing.T) {
	// Create mock VFS
	vfs := NewMockVFS(false)

	// Create FSProxy
	proxy := &FSProxyManager{
		vfs:     vfs,
		fdTable: NewFileDescriptorTable(),
	}

	// Test LLM_CHAT handler (fork+ExecuteInternal implementation)
	data := []byte("test.txt\nHello, World!")
	response := proxy.handleLLMChat(true, 0, 1, 2, data)

	// Verify response
	if response.Status != "OK" {
		t.Errorf("Expected status OK, got %s", response.Status)
	}

	// Response should contain size and quota status
	if !strings.Contains(response.Data, "weighted tokens") {
		t.Errorf("Expected response to contain quota status, got %s", response.Data)
	}
}

func TestLLMQuotaHandler(t *testing.T) {
	// Create mock VFS
	vfs := NewMockVFS(false)

	// Create FSProxy
	proxy := &FSProxyManager{
		vfs:     vfs,
		fdTable: NewFileDescriptorTable(),
	}

	// Test LLM_QUOTA handler
	response := proxy.handleLLMQuota()

	// Verify response
	if response.Status != "OK" {
		t.Errorf("Expected status OK, got %s", response.Status)
	}

	// Response should contain quota information
	if !strings.Contains(response.Data, "weighted tokens") {
		t.Errorf("Expected response to contain quota information, got %s", response.Data)
	}
}
