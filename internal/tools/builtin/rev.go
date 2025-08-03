package builtin

import (
	"bufio"
	"fmt"
	"io"
)

// Rev reverses each line
func Rev(args []string, stdin io.Reader, stdout io.Writer) error {
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
