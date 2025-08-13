package builtin

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"unicode"
)

// Wc counts lines, words, and characters
func Wc(args []string, stdin io.Reader, stdout io.Writer) error {
		if handled, _ := HandleHelp(args, stdout, `wc - Count lines, words, and characters

Usage: wc [options] [file...]

Options:
	-l                Count lines only
	-w                Count words only
	-c                Count characters/bytes only
	--help, -h        Show this help message

Examples:
	wc file.txt               Show lines, words, and characters
	wc -l file.txt            Count lines only
	wc -w file.txt            Count words only
`); handled {
				return nil
		}

	showLines := false
	showWords := false
	showBytes := false

	// Parse flags
	for _, arg := range args {
		switch arg {
		case "-l":
			showLines = true
		case "-w":
			showWords = true
		case "-c":
			showBytes = true
		}
	}

	// If no flags specified, show all
	if !showLines && !showWords && !showBytes {
		showLines = true
		showWords = true
		showBytes = true
	}

	processFunc := func(input io.Reader) error {
		lines := 0
		words := 0
		bytes := 0

		scanner := bufio.NewScanner(input)
		for scanner.Scan() {
			text := scanner.Text()
			lines++
			bytes += len(text) + 1 // +1 for newline

			// Count words
			fields := strings.FieldsFunc(text, unicode.IsSpace)
			words += len(fields)
		}

		if err := scanner.Err(); err != nil {
			return err
		}

		// Output results
		var result []string
		if showLines {
			result = append(result, fmt.Sprintf("%d", lines))
		}
		if showWords {
			result = append(result, fmt.Sprintf("%d", words))
		}
		if showBytes {
			result = append(result, fmt.Sprintf("%d", bytes))
		}

		fmt.Fprintln(stdout, strings.Join(result, " "))
		return nil
	}

	return processInput(args, stdin, processFunc)
}
