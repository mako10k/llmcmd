package builtin

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
)

// Head shows the first n lines of input (default: 10)
func Head(args []string, stdin io.Reader, stdout io.Writer) error {
	// Check for help option first
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			fmt.Fprint(stdout, `head - Display first lines of input

Usage: head [-n lines] [file...]

Options:
  -n lines          Number of lines to display (default: 10)
  --help, -h        Show this help message

Examples:
  head file.txt             Show first 10 lines
  head -n 5 file.txt        Show first 5 lines
`)
			return nil
		}
	}

	lines := 10

	// Parse number of lines from arguments
	for i, arg := range args {
		if arg == "-n" && i+1 < len(args) {
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				return fmt.Errorf("invalid number: %s", args[i+1])
			}
			if n < 0 {
				return fmt.Errorf("negative line count: %d", n)
			}
			lines = n
			// Remove processed arguments
			args = append(args[:i], args[i+2:]...)
			break
		}
	}

	processFunc := func(input io.Reader) error {
		scanner := bufio.NewScanner(input)
		for i := 0; i < lines && scanner.Scan(); i++ {
			fmt.Fprintln(stdout, scanner.Text())
		}
		return scanner.Err()
	}

	return processInput(args, stdin, processFunc)
}
