package builtin

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Tail outputs the last n lines (default 10)
func Tail(args []string, stdin io.Reader, stdout io.Writer) error {
	n := 10
	if len(args) > 0 && strings.HasPrefix(args[0], "-") {
		if val, err := strconv.Atoi(args[0][1:]); err == nil {
			n = val
		}
	}

	// Read all lines into memory
	var lines []string
	scanner := bufio.NewScanner(stdin)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Output last n lines
	start := len(lines) - n
	if start < 0 {
		start = 0
	}

	for i := start; i < len(lines); i++ {
		fmt.Fprintln(stdout, lines[i])
	}

	return nil
}
