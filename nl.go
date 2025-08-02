package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Nl numbers lines
func Nl(args []string, stdin io.Reader, stdout io.Writer) error {
	numberNonEmpty := false

	// Parse flags
	for _, arg := range args {
		switch arg {
		case "-b":
			numberNonEmpty = true
		}
	}

	scanner := bufio.NewScanner(stdin)
	lineNum := 1
	for scanner.Scan() {
		line := scanner.Text()

		if numberNonEmpty && strings.TrimSpace(line) == "" {
			fmt.Fprintln(stdout, line)
		} else {
			fmt.Fprintf(stdout, "%6d\t%s\n", lineNum, line)
			lineNum++
		}
	}

	return scanner.Err()
}
