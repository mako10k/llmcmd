package commands

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// SplitCommands contains file splitting and joining commands
type SplitCommands struct{}

// NewSplitCommands creates a new SplitCommands instance
func NewSplitCommands() *SplitCommands {
	return &SplitCommands{}
}

// ExecuteSplit implements split command
func (s *SplitCommands) ExecuteSplit(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	lines := 1000
	prefix := "x"
	byBytes := false

	// Parse arguments
	for i, arg := range args {
		if arg == "-l" && i+1 < len(args) {
			if l, err := strconv.Atoi(args[i+1]); err == nil {
				lines = l
			}
		}
		if arg == "-b" && i+1 < len(args) {
			byBytes = true
			if b, err := strconv.Atoi(args[i+1]); err == nil {
				lines = b // reuse lines variable for byte count
			}
		}
		// Last non-flag argument is prefix
		if !strings.HasPrefix(arg, "-") && i > 0 && !strings.HasPrefix(args[i-1], "-") {
			prefix = arg
		}
	}

	input, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("split: error reading input: %w", err)
	}

	if byBytes {
		return s.splitByBytes(input, lines, prefix, stdout)
	} else {
		return s.splitByLines(input, lines, prefix, stdout)
	}
}

// splitByLines splits input by line count
func (s *SplitCommands) splitByLines(input []byte, lineCount int, prefix string, stdout io.ReadWriteCloser) error {
	text := string(input)
	inputLines := strings.Split(text, "\n")

	fileNum := 0
	for i := 0; i < len(inputLines); i += lineCount {
		end := i + lineCount
		if end > len(inputLines) {
			end = len(inputLines)
		}

		// Generate filename: prefix + aa, ab, ac, etc.
		filename := s.generateFilename(prefix, fileNum)
		content := strings.Join(inputLines[i:end], "\n")

		// Output filename and content (simulated file write)
		_, err := stdout.Write([]byte(fmt.Sprintf("Creating %s:\n%s\n", filename, content)))
		if err != nil {
			return err
		}

		fileNum++
	}

	return nil
}

// splitByBytes splits input by byte count
func (s *SplitCommands) splitByBytes(input []byte, byteCount int, prefix string, stdout io.ReadWriteCloser) error {
	fileNum := 0
	for i := 0; i < len(input); i += byteCount {
		end := i + byteCount
		if end > len(input) {
			end = len(input)
		}

		// Generate filename: prefix + aa, ab, ac, etc.
		filename := s.generateFilename(prefix, fileNum)
		content := input[i:end]

		// Output filename and content (simulated file write)
		_, err := stdout.Write([]byte(fmt.Sprintf("Creating %s:\n%s\n", filename, string(content))))
		if err != nil {
			return err
		}

		fileNum++
	}

	return nil
}

// generateFilename generates split filenames (aa, ab, ac, ..., ba, bb, etc.)
func (s *SplitCommands) generateFilename(prefix string, num int) string {
	// Convert number to two-letter suffix
	first := 'a' + byte(num/26)
	second := 'a' + byte(num%26)
	return fmt.Sprintf("%s%c%c", prefix, first, second)
}

// ExecuteJoin implements join command
func (s *SplitCommands) ExecuteJoin(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	delimiter := " "
	field1 := 1
	field2 := 1

	// Parse arguments (simplified)
	for i, arg := range args {
		if arg == "-t" && i+1 < len(args) {
			delimiter = args[i+1]
		}
		if arg == "-1" && i+1 < len(args) {
			if f, err := strconv.Atoi(args[i+1]); err == nil {
				field1 = f
			}
		}
		if arg == "-2" && i+1 < len(args) {
			if f, err := strconv.Atoi(args[i+1]); err == nil {
				field2 = f
			}
		}
	}

	// For simplicity, we'll just demonstrate the concept
	// In a real implementation, join would read two files and join on specified fields
	input, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("join: error reading input: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(input)), "\n")

	// Simple join simulation - just output the lines with delimiter info
	_, err = stdout.Write([]byte(fmt.Sprintf("join: would join on field %d and %d with delimiter '%s'\n", field1, field2, delimiter)))
	if err != nil {
		return err
	}

	for _, line := range lines {
		_, err := stdout.Write([]byte(line + "\n"))
		if err != nil {
			return err
		}
	}

	return nil
}

// ExecuteComm implements comm command (compare sorted files)
func (s *SplitCommands) ExecuteComm(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	suppress1 := false
	suppress2 := false
	suppress3 := false

	// Parse arguments
	for _, arg := range args {
		if arg == "-1" {
			suppress1 = true
		}
		if arg == "-2" {
			suppress2 = true
		}
		if arg == "-3" {
			suppress3 = true
		}
	}

	// For simplicity, read all input and simulate comparison
	input, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("comm: error reading input: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(input)), "\n")

	// Simulate comm output format
	_, err = stdout.Write([]byte(fmt.Sprintf("comm: comparing lines (suppress: 1=%v, 2=%v, 3=%v)\n", suppress1, suppress2, suppress3)))
	if err != nil {
		return err
	}

	for _, line := range lines {
		// In real comm, this would compare two sorted files
		// For demo, just output the lines
		output := ""
		if !suppress1 {
			output += line + "\t"
		}
		if !suppress2 {
			output += "\t" + line
		}
		if !suppress3 {
			output += "\t\t" + line
		}

		_, err := stdout.Write([]byte(output + "\n"))
		if err != nil {
			return err
		}
	}

	return nil
}

// ExecuteCsplit implements csplit command (context split)
func (s *SplitCommands) ExecuteCsplit(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	// csplit is complex, so we'll implement a simplified version
	// For full implementation, we'd need to parse context patterns

	if len(args) == 0 {
		return fmt.Errorf("csplit: missing pattern")
	}

	pattern := args[0]
	prefix := "xx"

	// Parse prefix if provided
	for i, arg := range args {
		if arg == "-f" && i+1 < len(args) {
			prefix = args[i+1]
		}
	}

	input, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("csplit: error reading input: %w", err)
	}

	text := string(input)
	lines := strings.Split(text, "\n")

	// Simple pattern matching - split on lines containing pattern
	currentFile := 0
	currentContent := []string{}

	for _, line := range lines {
		if strings.Contains(line, pattern) && len(currentContent) > 0 {
			// Output current file
			filename := fmt.Sprintf("%s%02d", prefix, currentFile)
			content := strings.Join(currentContent, "\n")
			_, err := stdout.Write([]byte(fmt.Sprintf("Creating %s:\n%s\n", filename, content)))
			if err != nil {
				return err
			}
			currentFile++
			currentContent = []string{line}
		} else {
			currentContent = append(currentContent, line)
		}
	}

	// Output final file if there's remaining content
	if len(currentContent) > 0 {
		filename := fmt.Sprintf("%s%02d", prefix, currentFile)
		content := strings.Join(currentContent, "\n")
		_, err := stdout.Write([]byte(fmt.Sprintf("Creating %s:\n%s\n", filename, content)))
		if err != nil {
			return err
		}
	}

	return nil
}
