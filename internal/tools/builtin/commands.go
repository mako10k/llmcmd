package builtin

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

// VFS interface for file operations
type VFS interface {
	OpenFileSession(name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error)
}

var currentVFS VFS

// SetVFS sets the VFS instance for builtin commands
func SetVFS(vfs VFS) {
	currentVFS = vfs
}

// openFileForReading opens a file for reading, using VFS if available
func openFileForReading(filename string) (io.ReadCloser, error) {
	if currentVFS == nil {
		panic("SECURITY VIOLATION: VFS not initialized - builtin commands MUST use VFS for all file operations")
	}

	// Debug print
	fmt.Fprintf(os.Stderr, "DEBUG: Opening file for reading: %s\n", filename)

	// Normalize path to help injection gating
	if filename != "" && filename != "-" && filename[0] != '<' {
		if abs, err := filepath.Abs(filename); err == nil { filename = abs }
	}

	// Use external context (isInternal=false) to properly handle real files
	return currentVFS.OpenFileSession(filename, os.O_RDONLY, 0644)
}

// openFileForWriting opens a file for writing, using VFS if available
func openFileForWriting(filename string) (io.WriteCloser, error) {
	if currentVFS == nil {
		panic("SECURITY VIOLATION: VFS not initialized - builtin commands MUST use VFS for all file operations")
	}

	// Debug print
	fmt.Fprintf(os.Stderr, "DEBUG: Opening file for writing: %s\n", filename)

	// Use external context (isInternal=false) to properly handle real files
	return currentVFS.OpenFileSession(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
}

// processInput processes either stdin or files based on args
func processInput(args []string, stdin io.Reader, processor func(io.Reader) error) error {
	// Remove flags from args to get file arguments
	var files []string
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			files = append(files, arg)
		}
	}

	// If no files specified, process stdin
	if len(files) == 0 {
		return processor(stdin)
	}

	// Process each file
	for _, filename := range files {
		file, err := openFileForReading(filename)
		if err != nil {
			return fmt.Errorf("cannot open %s: %v", filename, err)
		}
		err = processor(file)
		file.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// CommandFunc represents a built-in command function
type CommandFunc func(args []string, stdin io.Reader, stdout io.Writer) error

// Commands maps command names to their implementations
var Commands = map[string]CommandFunc{
	"cat":    Cat,
	"grep":   Grep,
	"sed":    Sed,
	"head":   Head,
	"tail":   Tail,
	"sort":   Sort,
	"wc":     Wc,
	"tr":     Tr,
	"cut":    Cut,
	"uniq":   Uniq,
	"nl":     Nl,
	"tee":    Tee,
	"rev":    Rev,
	"diff":   Diff,
	"patch":  Patch,
	"help":   GetHelp,
	"man":    Man,
	"echo":   Echo,
	"llmcmd": Llmcmd,
	"llmsh":  Llmsh,
	"exit":   Exit,
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

// executeSubProcess executes a subprocess with error handling
func executeSubProcess(cmd *exec.Cmd, processName string) error {
	err := cmd.Run()
	if err != nil {
		// Extract exit code if available for better error reporting
		if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				exitCode := status.ExitStatus()
				return fmt.Errorf("%s process exited with code %d", processName, exitCode)
			}
		}
		return fmt.Errorf("%s fork execution failed: %v", processName, err)
	}
	return nil
}

// parseLineCountArg parses -n argument for head/tail commands
func parseLineCountArg(args []string, defaultLines int) (int, []string, error) {
	lines := defaultLines

	// Parse number of lines from arguments
	for i, arg := range args {
		if arg == "-n" && i+1 < len(args) {
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				return 0, args, fmt.Errorf("invalid number: %s", args[i+1])
			}
			if n < 0 {
				return 0, args, fmt.Errorf("negative line count: %d", n)
			}
			lines = n
			// Remove processed arguments
			args = append(args[:i], args[i+2:]...)
			break
		}
	}

	return lines, args, nil
}
