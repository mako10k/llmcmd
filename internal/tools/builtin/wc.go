package builtin

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Wc counts lines, words, and characters
func Wc(args []string, stdin io.Reader, stdout io.Writer) error {
	lines := 0
	words := 0
	chars := 0
	bytes := 0

	showLines := true
	showWords := true
	showChars := true
	showBytes := false

	// Parse flags
	flagCount := 0
	for _, arg := range args {
		switch arg {
		case "-l":
			if flagCount == 0 {
				showWords, showChars = false, false
			}
			showLines = true
			flagCount++
		case "-w":
			if flagCount == 0 {
				showLines, showChars = false, false
			}
			showWords = true
			flagCount++
		case "-c":
			if flagCount == 0 {
				showLines, showWords = false, false
			}
			showChars = true
			flagCount++
		case "-m":
			if flagCount == 0 {
				showLines, showWords = false, false
			}
			showBytes = true
			flagCount++
		}
	}

	scanner := bufio.NewScanner(stdin)
	for scanner.Scan() {
		line := scanner.Text()
		lines++
		chars += len([]rune(line)) + 1 // +1 for newline
		bytes += len(line) + 1

		// Count words
		wordScanner := bufio.NewScanner(strings.NewReader(line))
		wordScanner.Split(bufio.ScanWords)
		for wordScanner.Scan() {
			words++
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Output counts
	var output []string
	if showLines {
		output = append(output, fmt.Sprintf("%d", lines))
	}
	if showWords {
		output = append(output, fmt.Sprintf("%d", words))
	}
	if showBytes {
		output = append(output, fmt.Sprintf("%d", bytes))
	}
	if showChars && !showBytes {
		output = append(output, fmt.Sprintf("%d", chars))
	}

	fmt.Fprintln(stdout, strings.Join(output, " "))
	return nil
}
