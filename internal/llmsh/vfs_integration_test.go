package llmsh

import (
	"os"
	"testing"

	"github.com/mako10k/llmcmd/internal/app"
)

// TestFSProxyVFSIntegration tests the llmsh VFS with FSProxy integration
func TestFSProxyVFSIntegration(t *testing.T) {
	// Clean up any leftover test files
	defer func() {
		os.Remove("integration_test.txt")
		os.Remove("fsproxy_test.txt")
	}()

	// Test with FSProxy disabled (legacy mode)
	t.Run("LegacyMode", func(t *testing.T) {
		vfs := NewVirtualFileSystem([]string{}, []string{})
		defer vfs.CleanUp()

		// Test virtual file write and read cycle (not real file)
		virtualFileName := "virtual_test_file"
		writer, err := vfs.OpenForWrite(virtualFileName, false, false) // isTopLevelCmd=false to force virtual file
		if err != nil {
			t.Fatalf("Failed to open virtual file for writing: %v", err)
		}

		testData := "Testing FSProxy integration\n"
		_, err = writer.Write([]byte(testData))
		if err != nil {
			t.Fatalf("Failed to write data: %v", err)
		}
		// Don't close the writer, keep it open for read access

		// Read the data back from the same virtual file
		reader, err := vfs.OpenForRead(virtualFileName, false) // isTopLevelCmd=false to access virtual file
		if err != nil {
			t.Fatalf("Failed to open virtual file for reading: %v", err)
		}

		buffer := make([]byte, 1024)
		n, err := reader.Read(buffer)
		if err != nil && err.Error() != "EOF" {
			t.Fatalf("Failed to read data: %v", err)
		}

		result := string(buffer[:n])
		if result != testData {
			t.Errorf("Expected %q, got %q", testData, result)
		}

		// Clean up
		reader.Close()
		writer.Close()
	})

	// Test with FSProxy enabled
	t.Run("FSProxyMode", func(t *testing.T) {
		// Create a real file for FSProxy testing
		testFile := "fsproxy_test.txt"
		file, err := os.Create(testFile)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		_, err = file.WriteString("Hello, FSProxy VFS!")
		if err != nil {
			t.Fatalf("Failed to write to test file: %v", err)
		}
		file.Close()

		// Create FSProxy manager for testing (simplified approach)
		legacyVFS := NewVirtualFileSystem([]string{}, []string{})
		legacyAdapter := NewLegacyVFSAdapter(legacyVFS)

		// Create a pipe for FSProxy (though we won't use it in this test)
		pipeR, pipeW, err := os.Pipe()
		if err != nil {
			t.Fatalf("Failed to create pipe: %v", err)
		}
		defer pipeR.Close()
		defer pipeW.Close()

		fsProxyManager := app.NewFSProxyManager(legacyAdapter, pipeR, false)

		// Create VFS with FSProxy integration
		adapter := app.NewVFSFSProxyAdapter(fsProxyManager, legacyAdapter, true)
		vfs := NewVirtualFileSystemWithFSProxy([]string{}, []string{}, true, adapter)
		defer vfs.CleanUp()

		// Test reading through FSProxy
		reader, err := vfs.OpenForRead(testFile, true)
		if err != nil {
			t.Fatalf("Failed to open file for reading via FSProxy: %v", err)
		}

		buffer := make([]byte, 1024)
		n, err := reader.Read(buffer)
		if err != nil && err.Error() != "EOF" {
			t.Fatalf("Failed to read data via FSProxy: %v", err)
		}
		reader.Close()

		result := string(buffer[:n])
		expected := "Hello, FSProxy VFS!"
		if result != expected {
			t.Errorf("Expected %q, got %q", expected, result)
		}
	})
}
