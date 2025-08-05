package llmsh

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/mako10k/llmcmd/internal/app"
)

// TestVFSFSProxyIntegration tests the integration between llmsh VFS and FSProxy
func TestVFSFSProxyIntegration(t *testing.T) {
	// Create mock VFS for FSProxy using existing basic VFS implementation
	basicVFS := NewVirtualFileSystem([]string{}, []string{})
	mockVFS := NewLegacyVFSAdapter(basicVFS)

	// Create a dummy pipe for FSProxy (we won't actually use it in tests)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	// Create FSProxy manager with VFS mode enabled
	fsProxyManager := app.NewFSProxyManager(mockVFS, r, true)

	// Create test input and output files
	inputFiles := []string{"input.txt"}
	outputFiles := []string{"output.txt"}

	// Test Case 1: VFS with FSProxy disabled (legacy mode)
	t.Run("Legacy_Mode", func(t *testing.T) {
		config := &Config{
			InputFiles:     inputFiles,
			OutputFiles:    outputFiles,
			EnableFSProxy:  false,
			FSProxyManager: nil,
		}

		shell, err := NewShell(config)
		if err != nil {
			t.Fatalf("Failed to create shell: %v", err)
		}

		// Test file operations through VFS
		testContent := "Hello, legacy VFS!"

		// Write to virtual file
		writer, err := shell.vfs.OpenForWrite("test.txt", false, false)
		if err != nil {
			t.Fatalf("Failed to open file for writing: %v", err)
		}

		_, err = writer.Write([]byte(testContent))
		if err != nil {
			t.Fatalf("Failed to write to file: %v", err)
		}
		writer.Close()

		// Read from virtual file (open a new reader)
		reader, err := shell.vfs.OpenForRead("test.txt", false)
		if err != nil {
			t.Fatalf("Failed to open file for reading: %v", err)
		}

		var buf bytes.Buffer
		_, err = buf.ReadFrom(reader)
		if err != nil {
			t.Fatalf("Failed to read from file: %v", err)
		}
		reader.Close()

		if buf.String() != testContent {
			t.Errorf("Expected content '%s', got '%s'", testContent, buf.String())
		}
	})

	// Test Case 2: VFS with FSProxy enabled
	t.Run("FSProxy_Mode", func(t *testing.T) {
		config := &Config{
			InputFiles:     inputFiles,
			OutputFiles:    outputFiles,
			EnableFSProxy:  true,
			FSProxyManager: fsProxyManager,
		}

		shell, err := NewShell(config)
		if err != nil {
			t.Fatalf("Failed to create shell with FSProxy: %v", err)
		}

		// Verify FSProxy is enabled
		if !shell.vfs.enableFSProxy {
			t.Error("FSProxy should be enabled")
		}

		if shell.vfs.fsProxyAdapter == nil {
			t.Error("FSProxy adapter should be available")
		}

		// Test file operations through FSProxy
		testContent := "Hello, FSProxy VFS!"

		// Write to file through FSProxy
		writer, err := shell.vfs.OpenForWrite("fsproxy_test.txt", false, false)
		if err != nil {
			t.Fatalf("Failed to open file for writing through FSProxy: %v", err)
		}

		_, err = writer.Write([]byte(testContent))
		if err != nil {
			t.Fatalf("Failed to write to file through FSProxy: %v", err)
		}
		writer.Close()

		// Read from file through FSProxy
		reader, err := shell.vfs.OpenForRead("fsproxy_test.txt", false)
		if err != nil {
			t.Fatalf("Failed to open file for reading through FSProxy: %v", err)
		}

		var buf bytes.Buffer
		_, err = buf.ReadFrom(reader)
		if err != nil {
			t.Fatalf("Failed to read from file through FSProxy: %v", err)
		}
		reader.Close()

		if buf.String() != testContent {
			t.Errorf("Expected content '%s', got '%s'", testContent, buf.String())
		}
	})

	// Test Case 3: Command execution with FSProxy integration
	t.Run("Command_Execution_With_FSProxy", func(t *testing.T) {
		config := &Config{
			InputFiles:     inputFiles,
			OutputFiles:    outputFiles,
			EnableFSProxy:  true,
			FSProxyManager: fsProxyManager,
		}

		shell, err := NewShell(config)
		if err != nil {
			t.Fatalf("Failed to create shell with FSProxy: %v", err)
		}

		// Test simple command execution
		testScript := `echo "Testing FSProxy integration" > integration_test.txt`

		err = shell.Execute(testScript)
		if err != nil {
			t.Fatalf("Failed to execute command: %v", err)
		}

		// Verify output file was created and contains expected content
		reader, err := shell.vfs.OpenForRead("integration_test.txt", false)
		if err != nil {
			t.Fatalf("Failed to read integration test output: %v", err)
		}

		var buf bytes.Buffer
		_, err = buf.ReadFrom(reader)
		if err != nil {
			t.Fatalf("Failed to read content: %v", err)
		}
		reader.Close()

		content := strings.TrimSpace(buf.String())
		expectedContent := "Testing FSProxy integration"

		if content != expectedContent {
			t.Errorf("Expected content '%s', got '%s'", expectedContent, content)
		}
	})

	// Test Case 4: tools.VirtualFileSystem interface compliance
	t.Run("Tools_VFS_Interface_Compliance", func(t *testing.T) {
		config := &Config{
			InputFiles:     inputFiles,
			OutputFiles:    outputFiles,
			EnableFSProxy:  true,
			FSProxyManager: fsProxyManager,
		}

		shell, err := NewShell(config)
		if err != nil {
			t.Fatalf("Failed to create shell with FSProxy: %v", err)
		}

		// Test tools.VirtualFileSystem interface methods
		// OpenFile
		file, err := shell.vfs.OpenFile("tools_test.txt", os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			t.Fatalf("Failed to open file through tools interface: %v", err)
		}

		testContent := "Tools VFS interface test"
		_, err = file.Write([]byte(testContent))
		if err != nil {
			t.Fatalf("Failed to write through tools interface: %v", err)
		}
		file.Close()

		// CreateTemp
		tempFile, tempName, err := shell.vfs.CreateTemp("test")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}

		tempContent := "Temporary file content"
		_, err = tempFile.Write([]byte(tempContent))
		if err != nil {
			t.Fatalf("Failed to write to temp file: %v", err)
		}
		tempFile.Close()

		// ListFiles
		files := shell.vfs.ListFiles()
		if len(files) == 0 {
			t.Error("ListFiles should return non-empty list")
		}

		// RemoveFile
		err = shell.vfs.RemoveFile(tempName)
		if err != nil {
			t.Fatalf("Failed to remove temp file: %v", err)
		}
	})
}

// TestLegacyVFSAdapter tests the adapter between llmsh VFS and tools.VirtualFileSystem
func TestLegacyVFSAdapter(t *testing.T) {
	// Create a basic VFS
	vfs := NewVirtualFileSystem([]string{}, []string{})

	// Create adapter
	adapter := NewLegacyVFSAdapter(vfs)

	// Test adapter interface compliance
	testContent := "Adapter test content"

	// Test OpenFile
	file, err := adapter.OpenFile("adapter_test.txt", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open file through adapter: %v", err)
	}

	_, err = file.Write([]byte(testContent))
	if err != nil {
		t.Fatalf("Failed to write through adapter: %v", err)
	}
	file.Close()

	// Test CreateTemp
	tempFile, tempName, err := adapter.CreateTemp("adapter_test")
	if err != nil {
		t.Fatalf("Failed to create temp file through adapter: %v", err)
	}
	tempFile.Close()

	// Test ListFiles
	files := adapter.ListFiles()
	if len(files) == 0 {
		t.Error("Adapter should return file list")
	}

	// Test RemoveFile
	err = adapter.RemoveFile(tempName)
	if err != nil {
		t.Fatalf("Failed to remove file through adapter: %v", err)
	}
}
