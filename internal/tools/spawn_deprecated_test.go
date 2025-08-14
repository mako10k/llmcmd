package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestSpawnDeprecatedParams ensures legacy in_fd/out_fd are rejected explicitly.
func TestSpawnDeprecatedParams(t *testing.T) {
	engine, err := NewEngine(EngineConfig{})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	args := map[string]interface{}{
		"script": "echo test",
		"in_fd":  5,
	}
	b, _ := json.Marshal(args)
	toolCall := map[string]interface{}{
		"name":      "spawn",
		"arguments": string(b),
	}
	_, err = engine.ExecuteToolCall(toolCall)
	if err == nil {
		t.Fatalf("expected error for deprecated in_fd, got nil")
	}
	if !strings.Contains(err.Error(), "deprecated / forbidden parameter 'in_fd'") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
