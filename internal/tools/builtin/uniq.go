package builtin

import (
	"bufio"
	"fmt"
	"io"
)

// Uniq removes duplicate adjacent lines from input
func Uniq(args []string, stdin io.Reader, stdout io.Writer) error {
		if handled, _, err := HandleHelp(args, stdout, `uniq - Remove duplicate adjacent lines

Usage: uniq [options] [file...]

Options:
	-c                Prefix lines with occurrence count
	--help, -h        Show this help message

Examples:
	uniq file.txt             Remove duplicate adjacent lines
	uniq -c file.txt          Show count of occurrences
`); handled {
				return err
		}

	count := false

	// Parse flags
	for _, arg := range args {
		if arg == "-c" {
			count = true
		}
	}

	processFunc := func(input io.Reader) error {
		scanner := bufio.NewScanner(input)
		var lastLine string
		var lineCount int

		for scanner.Scan() {
			line := scanner.Text()

			if line != lastLine {
				// Print previous line if it exists
				if lastLine != "" {
					if count {
						fmt.Fprintf(stdout, "%6d %s\n", lineCount, lastLine)
					} else {
						fmt.Fprintln(stdout, lastLine)
					}
				}
				lastLine = line
				lineCount = 1
			} else {
				lineCount++
			}
		}

		// Print the last line
		if lastLine != "" {
			if count {
				fmt.Fprintf(stdout, "%6d %s\n", lineCount, lastLine)
			} else {
				fmt.Fprintln(stdout, lastLine)
			}
		}

		return scanner.Err()
	}

	return processInput(args, stdin, processFunc)
}
