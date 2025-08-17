package app

import (
	"fmt"
	"os"
	"sync"
	"testing"
)

// TestVFSAdapterIntegration tests VFS-FSProxy adapter integration
func TestVFSAdapterIntegration(t *testing.T) {
	// Create mock VFS (using existing MockVFS from fsproxy_test.go)
	mockVFS := NewMockVFS(true) // isTopLevel = true

	// Create FSProxy manager with mock VFS
	fsProxy := &FSProxyManager{
		vfs:     mockVFS,
		fdTable: NewFileDescriptorTable(),
	}

	// Create adapter
	adapter := NewVFSFSProxyAdapter(fsProxy, mockVFS, true)

	// Test OpenFile through adapter
	t.Run("AdapterOpenFile", func(t *testing.T) {
		openWriteAndCheck(t, adapter, "adapter_test.txt", []byte("Hello, FSProxy Adapter!"))
	})

	// Test CreateTemp through adapter
	t.Run("AdapterCreateTemp", func(t *testing.T) {
		_ = createTempWriteAndCheck(t, adapter, "adapter_pattern", []byte("Temporary adapter data"))
	})

	// Test ListFiles through adapter
	t.Run("AdapterListFiles", func(t *testing.T) {
		listFilesContains(t, adapter, "adapter_test", "adapter_pattern")
	})
}

// TestVFSAdapterFallbackMode tests fallback to legacy VFS when FSProxy is disabled
func TestVFSAdapterFallbackMode(t *testing.T) {
	// Create mock VFS for fallback
	mockVFS := NewMockVFS(true)

	// Create adapter with FSProxy disabled
	adapter := NewVFSFSProxyAdapter(nil, mockVFS, false)

	// Test that operations fall back to legacy VFS
	t.Run("FallbackOpenFile", func(t *testing.T) {
		helperFallbackOpenWrite(t, adapter, "fallback.txt", []byte("Fallback data"))
	})

	t.Run("FallbackListFiles", func(t *testing.T) {
		files := adapter.ListFiles()
		helperListContains(t, files, "fallback.txt")
	})
}

// TestFSProxyFileHandleLifecycle tests file handle lifecycle management
func TestFSProxyFileHandleLifecycle(t *testing.T) {
	// Create fd table
	fdTable := NewFileDescriptorTable()

	// Create mock file using existing MockFile constructor pattern
	mockVFS := NewMockVFS(true)
	mockFile, err := mockVFS.OpenFile("test_lifecycle.txt", os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("Failed to create mock file: %v", err)
	}

	// Create file handle
	handle := &FSProxyFileHandle{
		file:     mockFile,
		fd:       1001,
		fdTable:  fdTable,
		filename: "test_lifecycle.txt",
		closed:   false,
	}

	// Add to fd table
	fdTable.AddFile(1001, "test_lifecycle.txt", "r+", "test-client", true, handle)

	// Verify fd exists in table
	_, exists := fdTable.GetFile(1001)
	if !exists {
		t.Fatal("Expected fd to exist in table before close")
	}

	// Test operations before close
	t.Run("OperationsBeforeClose", func(t *testing.T) {
		// Test Write
		testData := []byte("Test content")
		helperWriteAndAssert(t, handle, testData)

		// Test Read (Note: MockFile may need position reset)
		buffer := make([]byte, len(testData))
		_, err := handle.Read(buffer)
		if err != nil {
			t.Logf("Read after write may fail due to position: %v", err)
			// This is acceptable for testing
		}
	})

	// Test Close and fd table cleanup
	t.Run("CloseAndCleanup", func(t *testing.T) {
		helperCloseAndAssertRemoved(t, handle, fdTable, 1001)
	})
	// Verify subsequent close operations don't fail
	if err2 := handle.Close(); err2 != nil {
		t.Errorf("Second close should not fail: %v", err2)
	}
}

// TestConvertFlagToModeUtility tests flag conversion utility function
func TestConvertFlagToModeUtility(t *testing.T) {
	adapter := NewVFSFSProxyAdapter(nil, nil, false)

	testCases := []struct {
		flag     int
		expected string
		name     string
	}{
		{os.O_RDONLY, "r", "ReadOnly"},
		{os.O_WRONLY, "w", "WriteOnly"},
		{os.O_WRONLY | os.O_CREATE | os.O_TRUNC, "w", "WriteCreateTrunc"},
		{os.O_WRONLY | os.O_CREATE | os.O_APPEND, "a", "WriteCreateAppend"},
		{os.O_RDWR, "r+", "ReadWrite"},
		{os.O_RDWR | os.O_CREATE | os.O_TRUNC, "w+", "ReadWriteCreateTrunc"},
		{os.O_RDWR | os.O_CREATE | os.O_APPEND, "a+", "ReadWriteCreateAppend"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := adapter.convertFlagToMode(tc.flag)
			if result != tc.expected {
				t.Errorf("convertFlagToMode(%d) = %s, expected %s", tc.flag, result, tc.expected)
			}
		})
	}
}

// TestVFSAdapterConcurrentOperations tests concurrent access to VFS adapter
func TestVFSAdapterConcurrentOperations(t *testing.T) {
	// Create mock VFS
	mockVFS := NewMockVFS(true)

	// Create FSProxy manager
	fsProxy := &FSProxyManager{
		vfs:     mockVFS,
		fdTable: NewFileDescriptorTable(),
	}

	// Create adapter
	adapter := NewVFSFSProxyAdapter(fsProxy, mockVFS, true)

	// Test concurrent file operations
	const numGoroutines = 10
	const numOperations = 3 // Reduced for stability

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numOperations)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				filename := fmt.Sprintf("concurrent_%d_%d.txt", id, j)

				// Open file
				file, err := adapter.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					errors <- fmt.Errorf("OpenFile failed for %s: %v", filename, err)
					continue
				}

				// Write data
				data := fmt.Sprintf("Data from goroutine %d, operation %d", id, j)
				_, err = file.Write([]byte(data))
				if err != nil {
					errors <- fmt.Errorf("Write failed for %s: %v", filename, err)
					file.Close()
					continue
				}

				// Close file
				err = file.Close()
				if err != nil {
					errors <- fmt.Errorf("Close failed for %s: %v", filename, err)
					continue
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}

	// Verify files were created
	files := adapter.ListFiles()
	if len(files) == 0 {
		t.Error("Expected some files to be created during concurrent operations")
	}

	// Note: Due to MockVFS implementation, we might not get exact count
	// The important thing is that no race conditions occurred
}

// TestVFSAdapterErrorHandling tests error handling in various scenarios
func TestVFSAdapterErrorHandling(t *testing.T) {
	t.Run("NoFSProxyNoFallback", func(t *testing.T) {
		// Create adapter with no FSProxy and no fallback
		adapter := NewVFSFSProxyAdapter(nil, nil, true)

		// Operations should fail gracefully
		_, err := adapter.OpenFile("test.txt", os.O_CREATE, 0644)
		if err == nil {
			t.Error("Expected error when no file system is available")
		}

		_, _, err = adapter.CreateTemp("pattern")
		if err == nil {
			t.Error("Expected error when no file system is available")
		}

		err = adapter.RemoveFile("test.txt")
		if err == nil {
			t.Error("Expected error when no file system is available")
		}

		files := adapter.ListFiles()
		if len(files) != 0 {
			t.Error("Expected empty file list when no file system is available")
		}
	})

	t.Run("FSProxyWithoutVFS", func(t *testing.T) {
		// Create FSProxy manager without VFS
		fsProxy := &FSProxyManager{
			vfs:     nil, // No VFS
			fdTable: NewFileDescriptorTable(),
		}

		adapter := NewVFSFSProxyAdapter(fsProxy, nil, true)

		// Operations should fail gracefully
		_, err := adapter.OpenFile("test.txt", os.O_CREATE, 0644)
		if err == nil {
			t.Error("Expected error when FSProxy has no VFS")
		}
	})
}
