package app

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestFSProxyAdvanced_QualityAssurance implements QA engineer's recommendations
func TestFSProxyAdvanced_QualityAssurance(t *testing.T) {
	t.Run("BoundaryValueTests", func(t *testing.T) {
		proxy := setupTestProxyWithMockVFS(t)

		t.Run("fd_maximum_allocation", func(t *testing.T) {
			// Test fd allocation up to reasonable limits
			var allocatedFDs []int

			for i := 0; i < 100; i++ { // Reasonable limit for testing
				response := proxy.handleOpen(fmt.Sprintf("test_file_%d.txt", i), "w", "true")
				if response.Status != "OK" {
					t.Logf("Failed to allocate fd at count %d: %s", i, response.Data)
					break
				}

				// Parse fd from response
				var fd int
				if n, err := fmt.Sscanf(response.Data, "%d", &fd); n != 1 || err != nil {
					t.Errorf("Failed to parse fd from response: %s", response.Data)
					continue
				}
				allocatedFDs = append(allocatedFDs, fd)
			}

			t.Logf("Successfully allocated %d file descriptors", len(allocatedFDs))

			// Cleanup - close all allocated fds
			for _, fd := range allocatedFDs {
				response := proxy.handleClose(fd)
				if response.Status != "OK" {
					t.Errorf("Failed to close fd %d: %s", fd, response.Data)
				}
			}
		})

		t.Run("zero_byte_operations", func(t *testing.T) {
			// Open file
			response := proxy.handleOpen("zero_test.txt", "w+", "true")
			if response.Status != "OK" {
				t.Fatalf("Failed to open file: %s", response.Data)
			}

			var fd int
			fmt.Sscanf(response.Data, "%d", &fd)

			// Test zero-byte read
			readResponse := proxy.handleRead(fd, 0, false)
			if readResponse.Status != "OK" {
				t.Errorf("Zero-byte read failed: %s", readResponse.Data)
			}

			// Test zero-byte write
			writeResponse := proxy.handleWrite(fd, []byte{})
			if writeResponse.Status != "OK" {
				t.Errorf("Zero-byte write failed: %s", writeResponse.Data)
			}

			// Cleanup
			proxy.handleClose(fd)
		})
	})

	t.Run("AbnormalCaseTests", func(t *testing.T) {
		proxy := setupTestProxyWithMockVFS(t)

		t.Run("invalid_fd_operations", func(t *testing.T) {
			invalidFDs := []int{-1, 0, 99999, 123456}

			for _, invalidFD := range invalidFDs {
				t.Run(fmt.Sprintf("invalid_fd_%d", invalidFD), func(t *testing.T) {
					// Test READ with invalid fd
					readResponse := proxy.handleRead(invalidFD, 10, false)
					if readResponse.Status != "ERROR" {
						t.Errorf("Expected ERROR for READ with invalid fd %d, got %s", invalidFD, readResponse.Status)
					}

					// Test WRITE with invalid fd
					writeResponse := proxy.handleWrite(invalidFD, []byte("test"))
					if writeResponse.Status != "ERROR" {
						t.Errorf("Expected ERROR for WRITE with invalid fd %d, got %s", invalidFD, writeResponse.Status)
					}

					// Test CLOSE with invalid fd
					closeResponse := proxy.handleClose(invalidFD)
					if closeResponse.Status != "ERROR" {
						t.Errorf("Expected ERROR for CLOSE with invalid fd %d, got %s", invalidFD, closeResponse.Status)
					}
				})
			}
		})

		t.Run("already_closed_fd_operations", func(t *testing.T) {
			// Open and close a file
			response := proxy.handleOpen("temp_close_test.txt", "w", "true")
			if response.Status != "OK" {
				t.Fatalf("Failed to open file: %s", response.Data)
			}

			var fd int
			fmt.Sscanf(response.Data, "%d", &fd)

			// Close the file
			closeResponse := proxy.handleClose(fd)
			if closeResponse.Status != "OK" {
				t.Fatalf("Failed to close file: %s", closeResponse.Data)
			}

			// Try to operate on closed fd
			readResponse := proxy.handleRead(fd, 10, false)
			if readResponse.Status != "ERROR" {
				t.Errorf("Expected ERROR for READ on closed fd, got %s", readResponse.Status)
			}

			writeResponse := proxy.handleWrite(fd, []byte("test"))
			if writeResponse.Status != "ERROR" {
				t.Errorf("Expected ERROR for WRITE on closed fd, got %s", writeResponse.Status)
			}

			// Second close should also fail
			closeResponse2 := proxy.handleClose(fd)
			if closeResponse2.Status != "ERROR" {
				t.Errorf("Expected ERROR for second CLOSE on same fd, got %s", closeResponse2.Status)
			}
		})
	})

	t.Run("ConcurrentAccessTests", func(t *testing.T) {
		proxy := setupTestProxyWithMockVFS(t)

		t.Run("concurrent_open_operations", func(t *testing.T) {
			const numGoroutines = 10
			const operationsPerGoroutine = 10

			var wg sync.WaitGroup
			results := make(chan string, numGoroutines*operationsPerGoroutine)

			// Launch concurrent OPEN operations
			for i := 0; i < numGoroutines; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()

					for j := 0; j < operationsPerGoroutine; j++ {
						filename := fmt.Sprintf("concurrent_file_%d_%d.txt", workerID, j)
						response := proxy.handleOpen(filename, "w", "true")
						results <- response.Status

						// If successful, close the file
						if response.Status == "OK" {
							var fd int
							fmt.Sscanf(response.Data, "%d", &fd)
							proxy.handleClose(fd)
						}
					}
				}(i)
			}

			wg.Wait()
			close(results)

			// Check results
			successCount := 0
			errorCount := 0
			for status := range results {
				if status == "OK" {
					successCount++
				} else {
					errorCount++
				}
			}

			t.Logf("Concurrent operations: %d successes, %d errors", successCount, errorCount)

			if successCount == 0 {
				t.Error("No successful concurrent operations - possible race condition")
			}
		})

		t.Run("concurrent_read_write_operations", func(t *testing.T) {
			// Open a file for concurrent access
			response := proxy.handleOpen("concurrent_rw_test.txt", "w+", "true")
			if response.Status != "OK" {
				t.Fatalf("Failed to open file for concurrent test: %s", response.Data)
			}

			var fd int
			fmt.Sscanf(response.Data, "%d", &fd)
			defer proxy.handleClose(fd)

			const numReaders = 5
			const numWriters = 5
			var wg sync.WaitGroup

			// Launch concurrent readers
			for i := 0; i < numReaders; i++ {
				wg.Add(1)
				go func(readerID int) {
					defer wg.Done()
					for j := 0; j < 5; j++ {
						proxy.handleRead(fd, 10, false)
						time.Sleep(time.Millisecond * 10)
					}
				}(i)
			}

			// Launch concurrent writers
			for i := 0; i < numWriters; i++ {
				wg.Add(1)
				go func(writerID int) {
					defer wg.Done()
					for j := 0; j < 5; j++ {
						data := []byte(fmt.Sprintf("writer_%d_op_%d\n", writerID, j))
						proxy.handleWrite(fd, data)
						time.Sleep(time.Millisecond * 10)
					}
				}(i)
			}

			wg.Wait()
			t.Log("Concurrent read/write operations completed without panic")
		})
	})

	t.Run("ResourceLeakTests", func(t *testing.T) {
		proxy := setupTestProxyWithMockVFS(t)

		t.Run("open_without_close_leak_detection", func(t *testing.T) {
			// Track initial state
			initialFiles := proxy.fdTable.GetAllFiles()
			initialCount := len(initialFiles)

			// Open multiple files without closing
			openedFDs := make([]int, 0, 50)
			for i := 0; i < 50; i++ {
				response := proxy.handleOpen(fmt.Sprintf("leak_test_%d.txt", i), "w", "true")
				if response.Status == "OK" {
					var fd int
					fmt.Sscanf(response.Data, "%d", &fd)
					openedFDs = append(openedFDs, fd)
				}
			}

			// Check that files are tracked
			currentFiles := proxy.fdTable.GetAllFiles()
			currentCount := len(currentFiles)

			expectedCount := initialCount + len(openedFDs)
			if currentCount != expectedCount {
				t.Errorf("Expected %d open files, got %d", expectedCount, currentCount)
			}

			// Simulate cleanup (would happen automatically on process exit)
			proxy.cleanup()

			// Check that cleanup removed all files
			afterCleanupFiles := proxy.fdTable.GetAllFiles()
			afterCleanupCount := len(afterCleanupFiles)

			if afterCleanupCount != 0 {
				t.Errorf("Expected 0 open files after cleanup, got %d", afterCleanupCount)
			}
		})
	})

	t.Run("ErrorHandlingTests", func(t *testing.T) {
		proxy := setupTestProxyWithMockVFS(t)

		t.Run("negative_size_read", func(t *testing.T) {
			// Open a file
			response := proxy.handleOpen("negative_size_test.txt", "w", "true")
			if response.Status != "OK" {
				t.Fatalf("Failed to open file: %s", response.Data)
			}

			var fd int
			fmt.Sscanf(response.Data, "%d", &fd)
			defer proxy.handleClose(fd)

			// Test negative size read
			readResponse := proxy.handleRead(fd, -1, false)
			if readResponse.Status != "ERROR" {
				t.Errorf("Expected ERROR for negative size read, got %s", readResponse.Status)
			}

			// Check error message is informative
			if readResponse.Data != "invalid size: negative value not allowed" {
				t.Errorf("Expected specific error message, got: %s", readResponse.Data)
			}
		})

		t.Run("is_top_level_flag_validation", func(t *testing.T) {
			// Test various is_top_level flag values
			testCases := []struct {
				flag          string
				shouldSucceed bool
			}{
				{"true", true},
				{"false", true},
				{"TRUE", false},
				{"FALSE", false},
				{"1", false},
				{"0", false},
				{"yes", false},
				{"no", false},
				{"", false},
				{"maybe", false},
			}

			for _, tc := range testCases {
				t.Run(fmt.Sprintf("flag_%s", tc.flag), func(t *testing.T) {
					response := proxy.handleOpen("flag_test.txt", "w", tc.flag)

					if tc.shouldSucceed {
						if response.Status != "OK" {
							t.Errorf("Expected OK for flag '%s', got %s: %s", tc.flag, response.Status, response.Data)
						}
						// Clean up if successful
						if response.Status == "OK" {
							var fd int
							fmt.Sscanf(response.Data, "%d", &fd)
							proxy.handleClose(fd)
						}
					} else {
						if response.Status != "ERROR" {
							t.Errorf("Expected ERROR for flag '%s', got %s", tc.flag, response.Status)
						}
					}
				})
			}
		})
	})
}

// Helper function to create proxy with mock VFS for testing
func setupTestProxyWithMockVFS(t *testing.T) *FSProxyManager {
	mockVFS := NewMockVFS(true) // Use the existing NewMockVFS function
	proxy := NewFSProxyManager(mockVFS, nil, true)
	return proxy
}
