package app

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestSpawnCommandMVP tests the minimum viable product spawn command implementation
func TestSpawnCommandMVP(t *testing.T) {
	fsProxy := &FSProxyManager{
		vfs:           NewMockVFS(false),
		fdTable:       NewFileDescriptorTable(),
		processTable:  NewProcessTable(),
	}

	t.Run("SuccessfulSpawn", func(t *testing.T) {
		// Test successful process spawn
		params := map[string]interface{}{
			"command": "echo",
			"args":    []string{"hello", "world"},
		}

		response := fsProxy.handleSpawn(params)

		// Verify response structure
		if response["status"] != "success" {
			t.Errorf("Expected status 'success', got %v", response["status"])
		}

		processID, ok := response["process_id"].(int)
		if !ok || processID <= 0 {
			t.Errorf("Expected valid process_id, got %v", response["process_id"])
		}

		// Verify process is registered in table
		process := fsProxy.processTable.GetProcess(processID)
		if process == nil {
			t.Error("Process not found in process table")
		}

		if process.GetStatus() != "running" {
			t.Errorf("Expected status 'running', got %s", process.GetStatus())
		}

		// Wait for process completion
		time.Sleep(100 * time.Millisecond)
		process = fsProxy.processTable.GetProcess(processID)
		if process.GetStatus() != "exited" {
			t.Errorf("Expected status 'exited', got %s", process.GetStatus())
		}
	})

	t.Run("InvalidCommand_FailFast", func(t *testing.T) {
		// Test Fail-First principle with invalid command
		params := map[string]interface{}{
			"command": "nonexistent_command_12345",
			"args":    []string{},
		}

		response := fsProxy.handleSpawn(params)

		// Verify immediate failure
		if response["status"] != "error" {
			t.Errorf("Expected status 'error' for invalid command, got %v", response["status"])
		}

		errorMsg, ok := response["error"].(string)
		if !ok || errorMsg == "" {
			t.Error("Expected error message for invalid command")
		}

		// Verify no process is registered
		if processID, exists := response["process_id"]; exists {
			t.Errorf("Invalid command should not create process, but got process_id: %v", processID)
		}
	})

	t.Run("EmptyCommand_FailFast", func(t *testing.T) {
		// Test Fail-First principle with empty command
		params := map[string]interface{}{
			"command": "",
			"args":    []string{},
		}

		response := fsProxy.handleSpawn(params)

		if response["status"] != "error" {
			t.Errorf("Expected status 'error' for empty command, got %v", response["status"])
		}
	})

	t.Run("ConcurrentSpawn", func(t *testing.T) {
		// Test concurrent spawn operations (race condition detection)
		var wg sync.WaitGroup
		results := make([]map[string]interface{}, 10)

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				params := map[string]interface{}{
					"command": "echo",
					"args":    []string{fmt.Sprintf("test%d", index)},
				}
				results[index] = fsProxy.handleSpawn(params)
			}(i)
		}

		wg.Wait()

		// Verify all spawns succeeded
		processIDs := make(map[int]bool)
		for i, result := range results {
			if result["status"] != "success" {
				t.Errorf("Concurrent spawn %d failed: %v", i, result)
			}

			processID := result["process_id"].(int)
			if processIDs[processID] {
				t.Errorf("Duplicate process ID detected: %d", processID)
			}
			processIDs[processID] = true
		}
	})
}

// TestProcessTableMVP tests the minimum viable product process table implementation
func TestProcessTableMVP(t *testing.T) {
	processTable := NewProcessTable()

	t.Run("AddAndGetProcess", func(t *testing.T) {
		process := &BackgroundProcess{
			PID:       12345,
			Command:   "test_command",
			Args:      []string{"arg1", "arg2"},
			Status:    "running",
			StartTime: time.Now(),
		}

		processTable.AddProcess(process)

		retrieved := processTable.GetProcess(12345)
		if retrieved == nil {
			t.Error("Failed to retrieve added process")
		}

		if retrieved.PID != 12345 || retrieved.Command != "test_command" {
			t.Error("Retrieved process data mismatch")
		}
	})

	t.Run("RemoveProcess", func(t *testing.T) {
		process := &BackgroundProcess{
			PID:     54321,
			Command: "remove_test",
			Status:  "exited",
		}

		processTable.AddProcess(process)
		processTable.RemoveProcess(54321)

		retrieved := processTable.GetProcess(54321)
		if retrieved != nil {
			t.Error("Process should be removed but still exists")
		}
	})

	t.Run("ListProcesses", func(t *testing.T) {
		// Clear existing processes
		processTable = NewProcessTable()

		// Add test processes
		for i := 1; i <= 3; i++ {
			process := &BackgroundProcess{
				PID:     i,
				Command: fmt.Sprintf("cmd%d", i),
				Status:  "running",
			}
			processTable.AddProcess(process)
		}

		processes := processTable.ListProcesses()
		if len(processes) != 3 {
			t.Errorf("Expected 3 processes, got %d", len(processes))
		}
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		// Test concurrent access to process table
		var wg sync.WaitGroup
		
		// Concurrent additions
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(pid int) {
				defer wg.Done()
				process := &BackgroundProcess{
					PID:     pid + 10000,
					Command: fmt.Sprintf("concurrent_cmd_%d", pid),
					Status:  "running",
				}
				processTable.AddProcess(process)
			}(i)
		}

		// Concurrent retrievals
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(pid int) {
				defer wg.Done()
				processTable.GetProcess(pid + 10000)
			}(i)
		}

		wg.Wait()

		// Verify final state
		processes := processTable.ListProcesses()
		if len(processes) < 100 {
			t.Errorf("Expected at least 100 processes, got %d", len(processes))
		}
	})
}

// TestBackgroundProcessMVP tests the minimum viable product BackgroundProcess structure
func TestBackgroundProcessMVP(t *testing.T) {
	t.Run("ProcessCreation", func(t *testing.T) {
		startTime := time.Now()
		process := &BackgroundProcess{
			PID:       999,
			Command:   "test_process",
			Args:      []string{"--verbose", "--output", "test.txt"},
			Status:    "running",
			StartTime: startTime,
		}

		if process.PID != 999 {
			t.Errorf("Expected PID 999, got %d", process.PID)
		}

		if process.GetStatus() != "running" {
			t.Errorf("Expected status 'running', got %s", process.GetStatus())
		}

		if process.StartTime != startTime {
			t.Error("StartTime mismatch")
		}
	})

	t.Run("StatusTransitions", func(t *testing.T) {
		process := &BackgroundProcess{
			PID:    123,
			Status: "running",
		}

		// Test status transition: running -> exited
		process.SetStatus("exited")
		process.EndTime = time.Now()

		if process.GetStatus() != "exited" {
			t.Errorf("Expected status 'exited', got %s", process.GetStatus())
		}

		if process.EndTime.IsZero() {
			t.Error("EndTime should be set when process exits")
		}
	})
}
