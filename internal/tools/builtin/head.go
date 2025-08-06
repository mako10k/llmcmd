package builtin

import (
	"bufio"
	"fmt"
	"io"

	"github.com/mako10k/llmcmd/internal/utils"
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

	lines, args, err := utils.ParseLineCountArgument(args, 10)
	if err != nil {
		return err
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
