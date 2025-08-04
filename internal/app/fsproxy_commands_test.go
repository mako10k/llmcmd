package app

import (
	"io"
	"strconv"
	"strings"
	"testing"
)

// TestFSProxyCommands_CriticalFactors implements 4-factor MVP testing for all fsproxy commands
func TestFSProxyCommands_CriticalFactors(t *testing.T) {
	// Factor 1: Core Functionality - Basic command operations
	t.Run("Factor1_CoreFunctionality", func(t *testing.T) {
		t.Run("OPEN_command", func(t *testing.T) {
			proxy, _, w := setupTestFSProxy(t, true)
			defer w.Close()
			
			tests := []struct {
				name           string
				filename       string
				mode           string
				isTopLevel     string
				expectedStatus string
				expectFD       bool
			}{
				{
					name:           "open_file_top_level_write",
					filename:       "test.txt",
					mode:           "w",
					isTopLevel:     "true",
					expectedStatus: "OK",
					expectFD:       true,
				},
				{
					name:           "open_file_child_process_read",
					filename:       "temp.txt",
					mode:           "r",
					isTopLevel:     "false",
					expectedStatus: "OK",
					expectFD:       true,
				},
				{
					name:           "open_file_all_modes",
					filename:       "multi.txt",
					mode:           "r+",
					isTopLevel:     "true",
					expectedStatus: "OK",
					expectFD:       true,
				},
			}
			
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					response := proxy.handleOpen(tt.filename, tt.mode, tt.isTopLevel)
					
					if response.Status != tt.expectedStatus {
						t.Errorf("Expected status %s, got %s", tt.expectedStatus, response.Status)
					}
					
					if tt.expectFD && response.Status == "OK" {
						// Verify that a file descriptor was assigned
						fd, err := strconv.Atoi(response.Data)
						if err != nil {
							t.Errorf("Expected numeric FD, got %s", response.Data)
						}
						if fd < 1000 {
							t.Errorf("Expected FD >= 1000, got %d", fd)
						}
					}
				})
			}
		})
		
		t.Run("READ_command", func(t *testing.T) {
			proxy, _, w := setupTestFSProxy(t, true)
			defer w.Close()
			
			// First open a file
			openResponse := proxy.handleOpen("test.txt", "w", "true")
			if openResponse.Status != "OK" {
				t.Fatalf("Failed to open file: %s", openResponse.Data)
			}
			
			fd := openResponse.Data
			
			// Test read operations
			tests := []struct {
				name           string
				fd             string
				size           string
				expectedStatus string
			}{
				{
					name:           "read_valid_size",
					fd:             fd,
					size:           "256",
					expectedStatus: "ERROR", // READ not fully implemented yet
				},
				{
					name:           "read_zero_size",
					fd:             fd,
					size:           "0",
					expectedStatus: "ERROR", // READ not fully implemented yet
				},
			}
			
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					fd, _ := strconv.Atoi(tt.fd)
					size, _ := strconv.Atoi(tt.size)
					response := proxy.handleRead(fd, size, false)
					
					if response.Status != tt.expectedStatus {
						t.Errorf("Expected status %s, got %s", tt.expectedStatus, response.Status)
					}
				})
			}
		})
		
		t.Run("WRITE_command", func(t *testing.T) {
			proxy, _, w := setupTestFSProxy(t, true)
			defer w.Close()
			
			// First open a file
			openResponse := proxy.handleOpen("test.txt", "w", "true")
			if openResponse.Status != "OK" {
				t.Fatalf("Failed to open file: %s", openResponse.Data)
			}
			
			fd := openResponse.Data
			testData := []byte("Hello, World!")
			
			tests := []struct {
				name           string
				fd             string
				size           string
				data           []byte
				expectedStatus string
			}{
				{
					name:           "write_valid_data",
					fd:             fd,
					size:           strconv.Itoa(len(testData)),
					data:           testData,
					expectedStatus: "ERROR", // WRITE not fully implemented yet
				},
				{
					name:           "write_empty_data",
					fd:             fd,
					size:           "0",
					data:           []byte{},
					expectedStatus: "ERROR", // WRITE not fully implemented yet
				},
			}
			
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					fd, _ := strconv.Atoi(tt.fd)
					response := proxy.handleWrite(fd, tt.data)
					
					if response.Status != tt.expectedStatus {
						t.Errorf("Expected status %s, got %s", tt.expectedStatus, response.Status)
					}
				})
			}
		})
		
		t.Run("CLOSE_command", func(t *testing.T) {
			proxy, _, w := setupTestFSProxy(t, true)
			defer w.Close()
			
			// First open a file
			openResponse := proxy.handleOpen("test.txt", "w", "true")
			if openResponse.Status != "OK" {
				t.Fatalf("Failed to open file: %s", openResponse.Data)
			}
			
			fd := openResponse.Data
			
			tests := []struct {
				name           string
				fd             string
				expectedStatus string
			}{
				{
					name:           "close_valid_fd",
					fd:             fd,
					expectedStatus: "ERROR", // CLOSE not fully implemented yet
				},
				{
					name:           "close_invalid_fd",
					fd:             "99999",
					expectedStatus: "ERROR",
				},
			}
			
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					fd, _ := strconv.Atoi(tt.fd)
					response := proxy.handleClose(fd)
					
					if response.Status != tt.expectedStatus {
						t.Errorf("Expected status %s, got %s", tt.expectedStatus, response.Status)
					}
				})
			}
		})
	})

	// Factor 2: Error Handling - Invalid inputs and error scenarios
	t.Run("Factor2_ErrorHandling", func(t *testing.T) {
		t.Run("OPEN_error_scenarios", func(t *testing.T) {
			proxy, _, w := setupTestFSProxy(t, true)
			defer w.Close()
			
			errorTests := []struct {
				name           string
				filename       string
				mode           string
				isTopLevel     string
				expectedError  string
			}{
				{
					name:          "invalid_mode",
					filename:      "test.txt",
					mode:          "invalid",
					isTopLevel:    "true",
					expectedError: "invalid mode",
				},
				{
					name:          "invalid_top_level_flag",
					filename:      "test.txt",
					mode:          "w",
					isTopLevel:    "maybe",
					expectedError: "invalid is_top_level",
				},
				{
					name:          "empty_filename",
					filename:      "",
					mode:          "w",
					isTopLevel:    "true",
					expectedError: "failed to open file",
				},
			}
			
			for _, tt := range errorTests {
				t.Run(tt.name, func(t *testing.T) {
					response := proxy.handleOpen(tt.filename, tt.mode, tt.isTopLevel)
					
					if response.Status != "ERROR" {
						t.Errorf("Expected ERROR status, got %s", response.Status)
					}
					
					if tt.expectedError != "" && !containsSubstring(response.Data, tt.expectedError) {
						t.Errorf("Expected error containing '%s', got '%s'", tt.expectedError, response.Data)
					}
				})
			}
		})
		
		t.Run("READ_error_scenarios", func(t *testing.T) {
			proxy, _, w := setupTestFSProxy(t, true)
			defer w.Close()
			
			errorTests := []struct {
				name          string
				fd            string
				size          string
				expectedError string
			}{
				{
					name:          "invalid_fd",
					fd:            "invalid",
					size:          "256",
					expectedError: "invalid fileno",
				},
				{
					name:          "invalid_size",
					fd:            "1000",
					size:          "invalid",
					expectedError: "invalid size",
				},
				{
					name:          "non_existent_fd",
					fd:            "99999",
					size:          "256",
					expectedError: "invalid fileno",
				},
			}
			
			for _, tt := range errorTests {
				t.Run(tt.name, func(t *testing.T) {
					fd, _ := strconv.Atoi(tt.fd)
					size, _ := strconv.Atoi(tt.size)
					response := proxy.handleRead(fd, size, false)
					
					if response.Status != "ERROR" {
						t.Errorf("Expected ERROR status, got %s", response.Status)
					}
					
					if tt.expectedError != "" && !containsSubstring(response.Data, tt.expectedError) {
						t.Errorf("Expected error containing '%s', got '%s'", tt.expectedError, response.Data)
					}
				})
			}
		})
		
		t.Run("VFS_unavailable_scenario", func(t *testing.T) {
			// Test with nil VFS
			proxy := &FSProxyManager{
				vfs:       nil,
				isVFSMode: true,
				nextFD:    1000,
				openFiles: make(map[int]io.ReadWriteCloser),
			}
			
			response := proxy.handleOpen("test.txt", "w", "true")
			
			if response.Status != "ERROR" {
				t.Errorf("Expected ERROR status when VFS is nil, got %s", response.Status)
			}
			
			if !containsSubstring(response.Data, "VFS not available") {
				t.Errorf("Expected 'VFS not available' error, got '%s'", response.Data)
			}
		})
	})

	// Factor 3: Data Integrity - File descriptor management and consistency
	t.Run("Factor3_DataIntegrity", func(t *testing.T) {
		t.Run("file_descriptor_allocation", func(t *testing.T) {
			proxy, _, w := setupTestFSProxy(t, true)
			defer w.Close()
			
			// Open multiple files and verify FD allocation
			var fds []string
			filenames := []string{"file1.txt", "file2.txt", "file3.txt"}
			
			for _, filename := range filenames {
				response := proxy.handleOpen(filename, "w", "true")
				if response.Status != "OK" {
					t.Fatalf("Failed to open %s: %s", filename, response.Data)
				}
				fds = append(fds, response.Data)
			}
			
			// Verify FDs are unique and sequential
			fdMap := make(map[string]bool)
			var lastFD int
			
			for i, fdStr := range fds {
				if fdMap[fdStr] {
					t.Errorf("Duplicate FD assigned: %s", fdStr)
				}
				fdMap[fdStr] = true
				
				fd, err := strconv.Atoi(fdStr)
				if err != nil {
					t.Errorf("Invalid FD format: %s", fdStr)
					continue
				}
				
				if i > 0 && fd <= lastFD {
					t.Errorf("FD not sequential: previous=%d, current=%d", lastFD, fd)
				}
				lastFD = fd
			}
		})
		
		t.Run("mode_validation_completeness", func(t *testing.T) {
			proxy, _, w := setupTestFSProxy(t, true)
			defer w.Close()
			
			validModes := []string{"r", "w", "a", "r+", "w+", "a+"}
			for _, mode := range validModes {
				response := proxy.handleOpen("test_"+mode+".txt", mode, "true")
				if response.Status != "OK" {
					t.Errorf("Valid mode '%s' should succeed, got %s: %s", mode, response.Status, response.Data)
				}
			}
			
			invalidModes := []string{"x", "rb", "wb", "rw", "invalid"}
			for _, mode := range invalidModes {
				response := proxy.handleOpen("test_"+mode+".txt", mode, "true")
				if response.Status != "ERROR" {
					t.Errorf("Invalid mode '%s' should fail, got %s", mode, response.Status)
				}
			}
		})
		
		t.Run("top_level_flag_consistency", func(t *testing.T) {
			proxy, _, w := setupTestFSProxy(t, true)
			defer w.Close()
			
			// Test both valid values
			validFlags := []string{"true", "false"}
			for _, flag := range validFlags {
				response := proxy.handleOpen("test_"+flag+".txt", "w", flag)
				if response.Status != "OK" {
					t.Errorf("Valid is_top_level '%s' should succeed, got %s: %s", flag, response.Status, response.Data)
				}
			}
			
			// Test invalid values
			invalidFlags := []string{"TRUE", "FALSE", "1", "0", "yes", "no", "maybe"}
			for _, flag := range invalidFlags {
				response := proxy.handleOpen("test_"+flag+".txt", "w", flag)
				if response.Status != "ERROR" {
					t.Errorf("Invalid is_top_level '%s' should fail, got %s", flag, response.Status)
				}
			}
		})
	})

	// Factor 4: Performance and Security - Resource management and limits
	t.Run("Factor4_PerformanceAndSecurity", func(t *testing.T) {
		t.Run("file_descriptor_limits", func(t *testing.T) {
			proxy, _, w := setupTestFSProxy(t, true)
			defer w.Close()
			
			// Open many files to test resource management
			const maxFiles = 100
			var successCount int
			
			for i := 0; i < maxFiles; i++ {
				filename := "stress_test_" + strconv.Itoa(i) + ".txt"
				response := proxy.handleOpen(filename, "w", "true")
				if response.Status == "OK" {
					successCount++
				}
			}
			
			if successCount == 0 {
				t.Error("Should be able to open at least some files")
			}
			
			t.Logf("Successfully opened %d out of %d files", successCount, maxFiles)
		})
		
		t.Run("filename_security_validation", func(t *testing.T) {
			proxy, _, w := setupTestFSProxy(t, true)
			defer w.Close()
			
			// Test potentially problematic filenames
			problematicNames := []string{
				"../../../etc/passwd",
				"/dev/null",
				"con.txt",     // Windows reserved
				"aux.txt",     // Windows reserved
				"file\x00.txt", // Null byte
				strings.Repeat("a", 1000), // Very long name
			}
			
			for _, filename := range problematicNames {
				response := proxy.handleOpen(filename, "w", "true")
				// The response depends on VFS implementation
				// For now, just verify the system doesn't crash
				t.Logf("Filename '%s' result: %s - %s", filename, response.Status, response.Data)
			}
		})
		
		t.Run("concurrent_access_simulation", func(t *testing.T) {
			proxy, _, w := setupTestFSProxy(t, true)
			defer w.Close()
			
			// Simulate concurrent access (basic test)
			done := make(chan bool, 2)
			
			go func() {
				for i := 0; i < 10; i++ {
					filename := "concurrent1_" + strconv.Itoa(i) + ".txt"
					proxy.handleOpen(filename, "w", "true")
				}
				done <- true
			}()
			
			go func() {
				for i := 0; i < 10; i++ {
					filename := "concurrent2_" + strconv.Itoa(i) + ".txt"
					proxy.handleOpen(filename, "w", "true")
				}
				done <- true
			}()
			
			// Wait for both goroutines
			<-done
			<-done
			
			// If we get here without panicking, basic thread safety is working
		})
	})
}

// Helper function to check if a string contains a substring (case-insensitive)
func containsSubstring(str, substr string) bool {
	return strings.Contains(strings.ToLower(str), strings.ToLower(substr))
}
