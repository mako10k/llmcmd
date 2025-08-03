package builtin

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// VFS interface for file operations
type VFS interface {
	OpenFileWithContext(name string, flag int, perm os.FileMode, isInternal bool) (io.ReadWriteCloser, error)
}

var currentVFS VFS

// SetVFS sets the VFS instance for builtin commands
func SetVFS(vfs VFS) {
	currentVFS = vfs
}

// openFileForReading opens a file for reading, using VFS if available
func openFileForReading(filename string) (io.ReadCloser, error) {
	if currentVFS != nil {
		return currentVFS.OpenFileWithContext(filename, os.O_RDONLY, 0644, true)
	}
	return os.Open(filename)
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
	"cat":        Cat,
	"grep":       Grep,
	"sed":        Sed,
	"head":       Head,
	"tail":       Tail,
	"sort":       Sort,
	"wc":         Wc,
	"tr":         Tr,
	"cut":        Cut,
	"uniq":       Uniq,
	"nl":         Nl,
	"tee":        Tee,
	"rev":        Rev,
	"diff":       Diff,
	"patch":      Patch,
	"help":       GetHelp,
	"echo":       Echo,
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
