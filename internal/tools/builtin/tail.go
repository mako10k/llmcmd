package builtin

import (
	"bufio"
	"fmt"
	"io"

	"github.com/mako10k/llmcmd/internal/utils"
)

// Tail shows the last n lines of input (default: 10)
func Tail(args []string, stdin io.Reader, stdout io.Writer) error {
	if handled, _, err := HandleHelp(args, stdout, `tail - Display last lines of input

Usage: tail [-n lines] [file...]

Options:
  -n lines          Number of lines to display (default: 10)
  --help, -h        Show this help message

Examples:
  tail file.txt             Show last 10 lines
  tail -n 5 file.txt        Show last 5 lines
`); handled {
		return err
	}

	lines, args, err := utils.ParseLineCountArgument(args, 10)
	if err != nil {
		return err
	}

	processFunc := func(input io.Reader) error {
		var buffer []string
		scanner := bufio.NewScanner(input)

		for scanner.Scan() {
			buffer = append(buffer, scanner.Text())
			if len(buffer) > lines {
				buffer = buffer[1:]
			}
		}

		for _, line := range buffer {
			fmt.Fprintln(stdout, line)
		}

		return scanner.Err()
	}

	return processInput(args, stdin, processFunc)
}
