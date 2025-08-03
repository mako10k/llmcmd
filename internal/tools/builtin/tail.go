package builtin

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
)

// Tail shows the last n lines of input (default: 10)
func Tail(args []string, stdin io.Reader, stdout io.Writer) error {
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
