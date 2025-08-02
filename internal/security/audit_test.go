package security

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestAuditEvent_CriticalFactors implements MVP 4-factor testing approach
func TestAuditEvent_CriticalFactors(t *testing.T) {
	// Factor 1: Core Functionality - AuditEvent Creation and Logging
	t.Run("Factor1_CoreFunctionality", func(t *testing.T) {
		tempDir := t.TempDir()
		logPath := filepath.Join(tempDir, "test_audit.log")

		logger, err := NewFileAuditLogger(logPath)
		if err != nil {
			t.Fatalf("Failed to create audit logger: %v", err)
		}
		defer logger.Close()

		event := AuditEvent{
			UserID:    "test_user",
			EventType: EventTypeAPIKeyUsage,
			Resource:  "api_key:sk-test",
			Action:    ActionCall,
			Details:   "API key validation",
			Success:   true,
		}

		err = logger.LogEvent(event)
		if err != nil {
			t.Fatalf("Failed to log event: %v", err)
		}

		// Verify log file was created and contains expected data
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			t.Fatal("Audit log file was not created")
		}

		content, err := ioutil.ReadFile(logPath)
		if err != nil {
			t.Fatalf("Failed to read audit log: %v", err)
		}

		if !strings.Contains(string(content), "test_user") {
			t.Error("Log does not contain expected user ID")
		}
		if !strings.Contains(string(content), EventTypeAPIKeyUsage) {
			t.Error("Log does not contain expected event type")
		}
	})

	// Factor 2: Error Handling - Failure scenarios and edge cases
	t.Run("Factor2_ErrorHandling", func(t *testing.T) {
		// Test with invalid log path
		logger, err := NewFileAuditLogger("/invalid/path/audit.log")
		if err == nil {
			t.Error("Expected error for invalid log path")
		}

		// Test logging to closed logger
		tempDir := t.TempDir()
		logPath := filepath.Join(tempDir, "test_audit.log")
		logger, err = NewFileAuditLogger(logPath)
		if err != nil {
			t.Fatalf("Failed to create audit logger: %v", err)
		}

		logger.Close()
		event := AuditEvent{
			UserID:    "test_user",
			EventType: EventTypeAPIKeyUsage,
			Action:    ActionCall,
			Success:   true,
		}

		err = logger.LogEvent(event)
		if err == nil {
			t.Error("Expected error when logging to closed logger")
		}
	})

	// Factor 3: Data Integrity - JSON format and required fields
	t.Run("Factor3_DataIntegrity", func(t *testing.T) {
		tempDir := t.TempDir()
		logPath := filepath.Join(tempDir, "test_audit.log")

		logger, err := NewFileAuditLogger(logPath)
		if err != nil {
			t.Fatalf("Failed to create audit logger: %v", err)
		}
		defer logger.Close()

		event := AuditEvent{
			UserID:    "test_user",
			EventType: EventTypeConfigAccess,
			Resource:  "/path/to/config",
			Action:    ActionRead,
			Details:   "Configuration file accessed",
			Success:   true,
		}

		err = logger.LogEvent(event)
		if err != nil {
			t.Fatalf("Failed to log event: %v", err)
		}

		// Read and parse the log entry as JSON
		content, err := ioutil.ReadFile(logPath)
		if err != nil {
			t.Fatalf("Failed to read audit log: %v", err)
		}

		lines := strings.Split(strings.TrimSpace(string(content)), "\n")
		if len(lines) == 0 {
			t.Fatal("No log entries found")
		}

		var loggedEvent map[string]interface{}
		err = json.Unmarshal([]byte(lines[0]), &loggedEvent)
		if err != nil {
			t.Fatalf("Failed to parse log entry as JSON: %v", err)
		}

		// Verify all required fields are present
		requiredFields := []string{"timestamp", "user_id", "event_type", "resource", "action", "details", "success"}
		for _, field := range requiredFields {
			if _, exists := loggedEvent[field]; !exists {
				t.Errorf("Required field '%s' missing from log entry", field)
			}
		}

		// Verify timestamp format
		if timestamp, ok := loggedEvent["timestamp"].(string); ok {
			_, err := time.Parse(time.RFC3339, timestamp)
			if err != nil {
				t.Errorf("Invalid timestamp format: %v", err)
			}
		} else {
			t.Error("Timestamp field is not a string")
		}
	})

	// Factor 4: Performance and Security - File permissions and concurrent access
	t.Run("Factor4_PerformanceAndSecurity", func(t *testing.T) {
		tempDir := t.TempDir()
		logPath := filepath.Join(tempDir, "test_audit.log")

		logger, err := NewFileAuditLogger(logPath)
		if err != nil {
			t.Fatalf("Failed to create audit logger: %v", err)
		}
		defer logger.Close()

		// Check file permissions (should be 0600 - owner read/write only)
		fileInfo, err := os.Stat(logPath)
		if err != nil {
			t.Fatalf("Failed to get file info: %v", err)
		}

		expectedPerm := os.FileMode(0600)
		if fileInfo.Mode().Perm() != expectedPerm {
			t.Errorf("Incorrect file permissions: got %v, expected %v", fileInfo.Mode().Perm(), expectedPerm)
		}

		// Test concurrent logging (basic thread safety)
		done := make(chan bool, 2)

		go func() {
			for i := 0; i < 10; i++ {
				event := AuditEvent{
					UserID:    "user1",
					EventType: EventTypeAPIKeyUsage,
					Action:    ActionCall,
					Success:   true,
				}
				logger.LogEvent(event)
			}
			done <- true
		}()

		go func() {
			for i := 0; i < 10; i++ {
				event := AuditEvent{
					UserID:    "user2",
					EventType: EventTypeFileIO,
					Action:    ActionRead,
					Success:   true,
				}
				logger.LogEvent(event)
			}
			done <- true
		}()

		// Wait for both goroutines to complete
		<-done
		<-done

		// Verify that all events were logged (20 total)
		content, err := ioutil.ReadFile(logPath)
		if err != nil {
			t.Fatalf("Failed to read audit log: %v", err)
		}

		lines := strings.Split(strings.TrimSpace(string(content)), "\n")
		if len(lines) != 20 {
			t.Errorf("Expected 20 log entries, got %d", len(lines))
		}
	})
}

// TestAuditManager_MVP tests the audit manager functionality
func TestAuditManager_MVP(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "audit_manager_test.log")

	manager, err := CreateAuditManagerFromConfig(logPath, "test_user")
	if err != nil {
		t.Fatalf("Failed to create audit manager: %v", err)
	}
	defer manager.Close()

	// Test different event types through manager
	testCases := []struct {
		name string
		fn   func()
	}{
		{
			name: "API Key Usage",
			fn:   func() { manager.LogAPIKeyUsage("sk-test", true, "API key validation successful") },
		},
		{
			name: "Config Access",
			fn:   func() { manager.LogConfigAccess("/home/user/.llmcmdrc", ActionRead, true, "Configuration loaded") },
		},
		{
			name: "File IO",
			fn:   func() { manager.LogFileIO("/tmp/test.txt", ActionWrite, true, "File written successfully") },
		},
		{
			name: "OpenAI Call",
			fn: func() {
				manager.LogOpenAICall("/v1/chat/completions", "gpt-4o-mini", true, "Chat completion successful")
			},
		},
		{
			name: "Tool Execution",
			fn:   func() { manager.LogToolExecution("cat", true, "File read completed") },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.fn()
		})
	}

	// Verify all events were logged
	content, err := ioutil.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read audit log: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != len(testCases) {
		t.Errorf("Expected %d log entries, got %d", len(testCases), len(lines))
	}
}

// TestDefaultAuditLogPath tests the default log path functionality
func TestDefaultAuditLogPath(t *testing.T) {
	path := GetDefaultAuditLogPath()

	// Should contain .llmcmd_audit.log
	if !strings.Contains(path, ".llmcmd_audit.log") {
		t.Errorf("Default audit log path should contain '.llmcmd_audit.log', got: %s", path)
	}

	// Should be absolute path (containing home directory or current directory)
	if !filepath.IsAbs(path) && !strings.HasPrefix(path, ".") {
		t.Errorf("Default audit log path should be absolute or relative, got: %s", path)
	}
}

// BenchmarkAuditLogging benchmarks the performance of audit logging
func BenchmarkAuditLogging(b *testing.B) {
	tempDir := b.TempDir()
	logPath := filepath.Join(tempDir, "benchmark_audit.log")

	logger, err := NewFileAuditLogger(logPath)
	if err != nil {
		b.Fatalf("Failed to create audit logger: %v", err)
	}
	defer logger.Close()

	event := AuditEvent{
		UserID:    "benchmark_user",
		EventType: EventTypeAPIKeyUsage,
		Resource:  "api_key:sk-benchmark",
		Action:    ActionCall,
		Details:   "Benchmark test event",
		Success:   true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.LogEvent(event)
	}
}
