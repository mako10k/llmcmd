package app

import (
	"strings"
	"testing"
)

func TestLLMChatHandler(t *testing.T) {
	// Create FSProxy via helper to ensure full initialization
	proxy, _, w := setupTestFSProxy(t, false)
	defer w.Close()

	// Inject a mock LLM client to avoid real API dependency
	proxy.SetLLMClient(newMockLLMClient())

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
	// Create FSProxy via helper to ensure full initialization
	proxy, _, w := setupTestFSProxy(t, false)
	defer w.Close()

	// Provide mock client so quota path uses client stats branch
	proxy.SetLLMClient(newMockLLMClient())
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
