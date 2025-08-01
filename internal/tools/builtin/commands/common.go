package commands

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sync"
)

// CommandFunc represents a built-in command function
type CommandFunc func(args []string, stdin io.Reader, stdout io.Writer) error

// VirtualFileSystem interface for builtin commands
type VirtualFileSystem interface {
	// New context-aware method
	OpenFileWithContext(filename string, mode string, isInternal bool) (interface{}, error)

	// Legacy compatibility methods
	OpenForRead(filename string) (io.ReadCloser, error)
	OpenForWrite(filename string, append bool) (io.WriteCloser, error)
	CreatePipe() (io.ReadCloser, io.WriteCloser, error)
	SetTopLevelMode(enabled bool)
	ListFiles() []string
	CleanUp() error

	// Deprecated methods (for compatibility)
	OpenFile(name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error)
	CreateTemp(pattern string) (io.ReadWriteCloser, string, error)
	RemoveFile(name string) error
}

// Global VFS context for builtin commands
var (
	globalVFS VirtualFileSystem
	vfsMutex  sync.RWMutex
)

// SetVFS sets the global VFS context for builtin commands
func SetVFS(vfs VirtualFileSystem) {
	vfsMutex.Lock()
	defer vfsMutex.Unlock()
	globalVFS = vfs
}

// GetVFS returns the current global VFS context
func GetVFS() VirtualFileSystem {
	vfsMutex.RLock()
	defer vfsMutex.RUnlock()
	return globalVFS
}

// Commands maps command names to their implementations
var Commands = map[string]CommandFunc{
	"echo": Echo,
}

// GetHelp returns help information for specified commands
func GetHelp(commands []string, stdin io.Reader, stdout io.Writer) error {
	// TODO: Implement proper help system when Help command is extracted
	fmt.Fprint(stdout, "Help system not yet implemented\n")
	return nil
}

// openFileForReading opens a file for reading with VFS support
func openFileForReading(filename string) (io.ReadCloser, error) {
	// Try VFS first if available
	if vfs := GetVFS(); vfs != nil {
		// Use OpenFileWithContext with isInternal=true for LLM access
		file, err := vfs.OpenFileWithContext(filename, "r", true)
		if err == nil {
			if readCloser, ok := file.(io.ReadCloser); ok {
				return readCloser, nil
			}
		}
		// VFS failed, fall through to real filesystem
	}

	// Fallback to real filesystem
	return os.Open(filename)
}

// compileRegex compiles a regex pattern and returns an error if invalid
func compileRegex(pattern string, ignoreCase bool) (*regexp.Regexp, error) {
	if ignoreCase {
		pattern = "(?i)" + pattern
	}
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %s", err)
	}
	return compiled, nil
}

// appendCount appends formatted count to output slice if condition is true
func appendCount(output []string, count int, condition bool) []string {
	if condition {
		return append(output, fmt.Sprintf("%d", count))
	}
	return output
}

// HelpFunc represents a command help function
type HelpFunc func() string

// checkForHelp checks if --help flag is present and displays help if so
func checkForHelp(args []string, helpFunc HelpFunc, stdout io.Writer) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			fmt.Fprint(stdout, helpFunc())
			return true
		}
	}
	return false
}

// removeHelpFlags removes --help and -h flags from args
func removeHelpFlags(args []string) []string {
	var filtered []string
	for _, arg := range args {
		if arg != "--help" && arg != "-h" {
			filtered = append(filtered, arg)
		}
	}
	return filtered
}
