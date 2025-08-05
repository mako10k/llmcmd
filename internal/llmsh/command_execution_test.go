package llmsh

import (
	"testing"

	"os"

	"github.com/mako10k/llmcmd/internal/app"
)

// TestLLMShCommandExecution tests command execution through llmsh with FSProxy
func TestLLMShCommandExecution(t *testing.T) {
	// Test basic echo command
	t.Run("EchoCommand", func(t *testing.T) {
		config := &Config{
			InputFiles:  []string{},
			OutputFiles: []string{},
		}

		shell, err := NewShell(config)
		if err != nil {
			t.Fatalf("Failed to create shell: %v", err)
		}

		// Parse and execute a simple echo command
		err = shell.Execute("echo 'Hello, llmsh!'")
		if err != nil {
			t.Fatalf("Failed to execute echo command: %v", err)
		}
		// Note: echo command should execute without error
		// Output verification would require capturing stdout
	})

	// Test with FSProxy enabled
	t.Run("FSProxyEnabledCommands", func(t *testing.T) {
		// Create FSProxy-enabled shell
		legacyVFS := NewVirtualFileSystem([]string{}, []string{})
		legacyAdapter := NewLegacyVFSAdapter(legacyVFS)

		// Create a pipe for FSProxy
		pipeR, pipeW, err := os.Pipe()
		if err != nil {
			t.Fatalf("Failed to create pipe: %v", err)
		}
		defer pipeR.Close()
		defer pipeW.Close()

		fsProxyManager := app.NewFSProxyManager(legacyAdapter, pipeR, false)

		config := &Config{
			InputFiles:     []string{},
			OutputFiles:    []string{},
			EnableFSProxy:  true,
			FSProxyManager: fsProxyManager,
		}

		shell, err := NewShell(config)
		if err != nil {
			t.Fatalf("Failed to create FSProxy-enabled shell: %v", err)
		}

		// Parse and execute commands through FSProxy
		err = shell.Execute("echo 'FSProxy integration working!'")
		if err != nil {
			t.Fatalf("Failed to execute command through FSProxy: %v", err)
		}
		// Note: Commands should execute without error through FSProxy
	})
}
