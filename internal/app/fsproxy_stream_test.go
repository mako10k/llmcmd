package app

import (
	"testing"
	"time"
)

// TestSpawnCommandIOStreams tests I/O stream creation and management
func TestSpawnCommandIOStreams(t *testing.T) {
	// Create FSProxy manager
	fsProxy := &FSProxyManager{
		processTable: NewProcessTable(),
	}

	t.Run("IOStreamCreation", func(t *testing.T) {
		// Test that I/O streams are properly created and attached
		params := map[string]interface{}{
			"command": "echo",
			"args":    []string{"test"},
		}

		response := fsProxy.handleSpawn(params)

		// Verify successful spawn
		if status, ok := response["status"].(string); !ok || status != "success" {
			t.Errorf("Expected successful spawn, got: %v", response)
		}

		processID, ok := response["process_id"].(int)
		if !ok {
			t.Error("process_id not returned")
		}

		// Get the process and verify I/O streams are created
		process := fsProxy.processTable.GetProcess(processID)
		if process == nil {
			t.Error("Process not found in process table")
		}

		// Verify I/O streams are not nil
		if process.Stdin == nil {
			t.Error("Stdin stream not created")
		}
		if process.Stdout == nil {
			t.Error("Stdout stream not created")
		}
		if process.Stderr == nil {
			t.Error("Stderr stream not created")
		}

		// Wait for process completion to verify cleanup
		time.Sleep(100 * time.Millisecond)
		
		// Process should have completed successfully
		if process.GetStatus() != "exited" {
			t.Errorf("Expected status 'exited', got %s", process.GetStatus())
		}
	})

	t.Run("IOStreamFailureHandling", func(t *testing.T) {
		// Test with invalid command to check pipe cleanup on failure
		params := map[string]interface{}{
			"command": "nonexistent_command_that_should_fail",
			"args":    []string{},
		}

		response := fsProxy.handleSpawn(params)

		// Should fail and return error
		if status, ok := response["status"].(string); !ok || status != "error" {
			t.Errorf("Expected error status, got: %v", response)
		}

		// Error message should indicate process start failure
		if errorMsg, ok := response["error"].(string); !ok || errorMsg == "" {
			t.Error("Expected error message")
		}
	})

	t.Run("IOStreamConcurrentCreation", func(t *testing.T) {
		// Test concurrent process creation with I/O streams
		done := make(chan bool, 5)

		for i := 0; i < 5; i++ {
			go func() {
				params := map[string]interface{}{
					"command": "echo",
					"args":    []string{"concurrent"},
				}

				response := fsProxy.handleSpawn(params)
				if status, ok := response["status"].(string); !ok || status != "success" {
					t.Errorf("Concurrent spawn failed: %v", response)
				}
				done <- true
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 5; i++ {
			<-done
		}

		// Allow processes to complete
		time.Sleep(200 * time.Millisecond)
	})
}

// TestBackgroundProcessIOFields tests the new I/O stream fields
func TestBackgroundProcessIOFields(t *testing.T) {
	t.Run("IOFieldsInitialization", func(t *testing.T) {
		// Create a BackgroundProcess and verify I/O fields can be set
		process := &BackgroundProcess{
			PID:       999,
			Command:   "test",
			Args:      []string{"arg1"},
			Status:    "running",
			StartTime: time.Now(),
		}

		// I/O fields should be nil by default
		if process.Stdin != nil {
			t.Error("Stdin should be nil by default")
		}
		if process.Stdout != nil {
			t.Error("Stdout should be nil by default")
		}
		if process.Stderr != nil {
			t.Error("Stderr should be nil by default")
		}

		// Test that the structure can accommodate I/O fields
		// (This test ensures the fields exist and are properly typed)
		var stdin, stdout, stderr interface{}
		stdin = process.Stdin
		stdout = process.Stdout
		stderr = process.Stderr

		if stdin == nil && stdout == nil && stderr == nil {
			// Expected behavior - fields exist and are nil by default
		}
	})
}
