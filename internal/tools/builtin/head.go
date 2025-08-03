package builtin

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
)

// Head shows the first n lines of input (default: 10)
func Head(args []string, stdin io.Reader, stdout io.Writer) error {
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
