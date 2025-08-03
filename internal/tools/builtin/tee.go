package builtin

import (
	"bufio"
	"fmt"
	"io"
)

// Tee writes input to both stdout and multiple files
func Tee(args []string, stdin io.Reader, stdout io.Writer) error {
	// Check for help option first
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			fmt.Fprint(stdout, `tee - Write input to stdout and files

Usage: tee [file...]

Description:
  Copy input to standard output and to files (security: stdout only)

Options:
  --help, -h        Show this help message

Examples:
  tee                       Copy input to stdout only
  echo "data" | tee         Display and copy data
`)
			return nil
		}
	}

	// For security, we only support writing to stdout
	// File writing should be handled by the main write tool

	processFunc := func(input io.Reader) error {
		scanner := bufio.NewScanner(input)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintln(stdout, line)
		}
		return scanner.Err()
	}

	return processInput(args, stdin, processFunc)
}
