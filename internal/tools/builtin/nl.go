package builtin

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Nl numbers lines
func Nl(args []string, stdin io.Reader, stdout io.Writer) error {
	if handled, _, err := HandleHelp(args, stdout, `nl - Number lines

Usage: nl [options] [file...]

Options:
	-b                Number non-empty lines only
	--help, -h        Show this help message

Examples:
	nl file.txt               Number all lines
	nl -b file.txt            Number non-empty lines only
`); handled {
		return err
	}

	numberNonEmpty := false

	// Parse flags
	for _, arg := range args {
		switch arg {
		case "-b":
			numberNonEmpty = true
		}
	}

	processFunc := func(input io.Reader) error {
		scanner := bufio.NewScanner(input)
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

	return processInput(args, stdin, processFunc)
}
