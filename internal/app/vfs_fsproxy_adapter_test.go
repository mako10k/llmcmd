package app

import (
	"os"
	"strings"
	"testing"
)

// TestVFSFSProxyAdapter tests the VFS-FSProxy adapter functionality
func TestVFSFSProxyAdapter(t *testing.T) {
	// Create mock VFS (not top-level for testing)
	mockVFS := NewMockVFS(false)

	// Create mock FSProxy manager (minimal for testing)
	fsProxy := &FSProxyManager{
		vfs:     mockVFS,
		fdTable: NewFileDescriptorTable(),
	}

	// Create adapter
	adapter := NewVFSFSProxyAdapter(fsProxy, mockVFS, true)

	// Test OpenFile
	t.Run("OpenFile", func(t *testing.T) {
		file, err := adapter.OpenFile("test.txt", os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			t.Fatalf("OpenFile failed: %v", err)
		}
		defer file.Close()

		// Write some data
		_, err = file.Write([]byte("Hello, World!"))
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	})

	// Test CreateTemp
	t.Run("CreateTemp", func(t *testing.T) {
		file, filename, err := adapter.CreateTemp("testpattern")
		if err != nil {
			t.Fatalf("CreateTemp failed: %v", err)
		}
		defer file.Close()

		if !strings.Contains(filename, "testpattern") {
			t.Errorf("Expected filename to contain 'testpattern', got: %s", filename)
		}

		// Write to temp file
		_, err = file.Write([]byte("Temporary data"))
		if err != nil {
			t.Fatalf("Write to temp file failed: %v", err)
		}
	})

	// Test ListFiles
	t.Run("ListFiles", func(t *testing.T) {
		files := adapter.ListFiles()
		if len(files) == 0 {
			t.Error("Expected some files to be listed")
		}
	})

	// Test RemoveFile
	t.Run("RemoveFile", func(t *testing.T) {
		err := adapter.RemoveFile("test.txt")
		if err != nil {
			t.Fatalf("RemoveFile failed: %v", err)
		}
	})
}

// TestVFSFSProxyAdapterFallback tests fallback to legacy VFS
func TestVFSFSProxyAdapterFallback(t *testing.T) {
	// Create mock VFS for fallback
	mockVFS := NewMockVFS(false)

	// Create adapter with FSProxy disabled
	adapter := NewVFSFSProxyAdapter(nil, mockVFS, false)

	// Test that operations fall back to legacy VFS
	t.Run("FallbackOpenFile", func(t *testing.T) {
		file, err := adapter.OpenFile("fallback.txt", os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			t.Fatalf("Fallback OpenFile failed: %v", err)
		}
		defer file.Close()

		_, err = file.Write([]byte("Fallback data"))
		if err != nil {
			t.Fatalf("Write to fallback file failed: %v", err)
		}
	})

	t.Run("FallbackListFiles", func(t *testing.T) {
		files := adapter.ListFiles()
		found := false
		for _, file := range files {
			if file == "fallback.txt" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected fallback.txt to be in file list")
		}
	})
}

// TestFSProxyFileHandle tests the file handle wrapper
func TestFSProxyFileHandle(t *testing.T) {
	// Create mock file
	mockFile := &MockFile{
		content:  []byte("Test content"),
		position: 0,
		closed:   false,
	}

	// Create fd table
	fdTable := NewFileDescriptorTable()

	// Create file handle
	handle := &FSProxyFileHandle{
		file:     mockFile,
		fd:       1001,
		fdTable:  fdTable,
		filename: "test.txt",
		closed:   false,
	}

	// Add to fd table
	fdTable.AddFile(1001, "test.txt", "r+", "test-client", true, handle)

	// Test Read
	t.Run("Read", func(t *testing.T) {
		buffer := make([]byte, 12)
		n, err := handle.Read(buffer)
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}
		if n != 12 {
			t.Errorf("Expected to read 12 bytes, got %d", n)
		}
		if string(buffer) != "Test content" {
			t.Errorf("Expected 'Test content', got '%s'", string(buffer))
		}
	})

	// Test Write
	t.Run("Write", func(t *testing.T) {
		data := []byte(" - additional")
		n, err := handle.Write(data)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		if n != len(data) {
			t.Errorf("Expected to write %d bytes, got %d", len(data), n)
		}
	})

	// Test Close and fd table cleanup
	t.Run("Close", func(t *testing.T) {
		err := handle.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}

		// Verify fd was removed from table
		_, exists := fdTable.GetFile(1001)
		if exists {
			t.Error("Expected fd to be removed from table after close")
		}

		// Verify subsequent operations fail
		_, err = handle.Read(make([]byte, 10))
		if err == nil {
			t.Error("Expected read to fail after close")
		}
	})
}

// TestConvertFlagToMode tests flag conversion
func TestConvertFlagToMode(t *testing.T) {
	adapter := NewVFSFSProxyAdapter(nil, nil, false)

	testCases := []struct {
		flag     int
		expected string
	}{
		{os.O_RDONLY, "r"},
		{os.O_WRONLY, "w"},
		{os.O_WRONLY | os.O_CREATE | os.O_TRUNC, "w"},
		{os.O_WRONLY | os.O_CREATE | os.O_APPEND, "a"},
		{os.O_RDWR, "r+"},
		{os.O_RDWR | os.O_CREATE | os.O_TRUNC, "w+"},
		{os.O_RDWR | os.O_CREATE | os.O_APPEND, "a+"},
	}

	for _, tc := range testCases {
		result := adapter.convertFlagToMode(tc.flag)
		if result != tc.expected {
			t.Errorf("convertFlagToMode(%d) = %s, expected %s", tc.flag, result, tc.expected)
		}
	}
}
