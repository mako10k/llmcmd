package builtin

import (
	"bufio"
	"fmt"
	"io"
)

// Rev reverses each line
func Rev(args []string, stdin io.Reader, stdout io.Writer) error {
	if handled, _, err := HandleHelp(args, stdout, `rev - Reverse lines character by character

Usage: rev [file...]

Description:
	Reverse the order of characters in each line

Options:
	--help, -h        Show this help message

Examples:
	rev file.txt              Reverse each line in file
	echo "hello" | rev        Output: "olleh"
`); handled {
		return err
	}

	processFunc := func(input io.Reader) error {
		scanner := bufio.NewScanner(input)
		for scanner.Scan() {
			line := scanner.Text()
			runes := []rune(line)

			// Reverse the runes
			for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
				runes[i], runes[j] = runes[j], runes[i]
			}

			fmt.Fprintln(stdout, string(runes))
		}
		return scanner.Err()
	}

	return processInput(args, stdin, processFunc)
}
