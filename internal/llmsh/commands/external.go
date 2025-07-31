package commands

import (
	"fmt"
	"io"
	"os/exec"
)

// ExternalCommands handles commands that require external execution
// This is for complex commands that are difficult to implement internally
type ExternalCommands struct{}

// NewExternalCommands creates a new ExternalCommands instance
func NewExternalCommands() *ExternalCommands {
	return &ExternalCommands{}
}

// ExecuteExternal executes an external command with safety checks
func (e *ExternalCommands) ExecuteExternal(name string, args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	// Whitelist of allowed external commands for security
	allowedCommands := map[string]bool{
		"gzip":    true,
		"gunzip":  true,
		"bzip2":   true,
		"bunzip2": true,
		"xz":      true,
		"unxz":    true,
	}

	if !allowedCommands[name] {
		return fmt.Errorf("external command not allowed: %s", name)
	}

	// Create command
	cmd := exec.Command(name, args...)

	// Connect pipes
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stdout // Redirect stderr to stdout for simplicity

	// Execute
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("external command failed: %s: %w", name, err)
	}

	return nil
}

// ExecuteExternalGzip executes external gzip command
func (e *ExternalCommands) ExecuteExternalGzip(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	return e.ExecuteExternal("gzip", args, stdin, stdout)
}

// ExecuteExternalGunzip executes external gunzip command
func (e *ExternalCommands) ExecuteExternalGunzip(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	return e.ExecuteExternal("gunzip", args, stdin, stdout)
}

// ExecuteExternalBzip2 executes external bzip2 command
func (e *ExternalCommands) ExecuteExternalBzip2(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	return e.ExecuteExternal("bzip2", args, stdin, stdout)
}

// ExecuteExternalBunzip2 executes external bunzip2 command
func (e *ExternalCommands) ExecuteExternalBunzip2(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	return e.ExecuteExternal("bunzip2", args, stdin, stdout)
}

// ExecuteExternalXz executes external xz command
func (e *ExternalCommands) ExecuteExternalXz(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	return e.ExecuteExternal("xz", args, stdin, stdout)
}

// ExecuteExternalUnxz executes external unxz command
func (e *ExternalCommands) ExecuteExternalUnxz(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	return e.ExecuteExternal("unxz", args, stdin, stdout)
}
