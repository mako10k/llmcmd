package builtin

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Tr translates or deletes characters
func Tr(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) < 1 {
		return fmt.Errorf("tr: missing operand")
	}

	delete := false
	if args[0] == "-d" {
		delete = true
		args = args[1:]
		if len(args) < 1 {
			return fmt.Errorf("tr: missing character set")
		}
	}

	if delete {
		// Delete characters mode
		deleteSet := args[0]
		deleteRunes := make(map[rune]bool)
		for _, r := range deleteSet {
			deleteRunes[r] = true
		}

		scanner := bufio.NewScanner(stdin)
		for scanner.Scan() {
			line := scanner.Text()
			var result strings.Builder
			for _, r := range line {
				if !deleteRunes[r] {
					result.WriteRune(r)
				}
			}
			fmt.Fprintln(stdout, result.String())
		}
		return scanner.Err()
	}

	// Translation mode
	if len(args) < 2 {
		return fmt.Errorf("tr: missing replacement set")
	}

	fromSet := args[0]
	toSet := args[1]

	fromRunes := []rune(fromSet)
	toRunes := []rune(toSet)

	// Create translation map
	translation := make(map[rune]rune)
	for i, r := range fromRunes {
		if i < len(toRunes) {
			translation[r] = toRunes[i]
		} else if len(toRunes) > 0 {
			// Use last character for remaining translations
			translation[r] = toRunes[len(toRunes)-1]
		}
	}

	scanner := bufio.NewScanner(stdin)
	for scanner.Scan() {
		line := scanner.Text()
		var result strings.Builder
		for _, r := range line {
			if newR, exists := translation[r]; exists {
				result.WriteRune(newR)
			} else {
				result.WriteRune(r)
			}
		}
		fmt.Fprintln(stdout, result.String())
	}

	return scanner.Err()
}
