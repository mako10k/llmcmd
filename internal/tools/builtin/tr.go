package builtin

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Tr translates or deletes characters
func Tr(args []string, stdin io.Reader, stdout io.Writer) error {
	// Check for help option first
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			fmt.Fprint(stdout, `tr - Translate or delete characters

Usage: tr [options] set1 [set2]
       tr -d set1

Options:
  -d                Delete characters in set1
  --help, -h        Show this help message

Examples:
  tr 'a-z' 'A-Z'            Convert lowercase to uppercase
  tr -d '0-9'               Delete all digits
  tr ' ' '_'                Replace spaces with underscores
`)
			return nil
		}
	}

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

		processFunc := func(input io.Reader) error {
			scanner := bufio.NewScanner(input)
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

		// Remove -d and character set from args
		var remainingArgs []string
		if len(args) > 1 {
			remainingArgs = args[1:]
		}
		return processInput(remainingArgs, stdin, processFunc)
	}

	// Translation mode
	if len(args) < 2 {
		return fmt.Errorf("tr: missing operands")
	}

	set1 := args[0]
	set2 := args[1]

	// Create translation map
	replaceMap := make(map[rune]rune)
	runes1 := []rune(set1)
	runes2 := []rune(set2)

	minLen := len(runes1)
	if len(runes2) < minLen {
		minLen = len(runes2)
	}

	for i := 0; i < minLen; i++ {
		replaceMap[runes1[i]] = runes2[i]
	}

	processFunc := func(input io.Reader) error {
		scanner := bufio.NewScanner(input)
		for scanner.Scan() {
			line := scanner.Text()
			var result strings.Builder

			for _, r := range line {
				if replacement, exists := replaceMap[r]; exists {
					result.WriteRune(replacement)
				} else {
					result.WriteRune(r)
				}
			}

			fmt.Fprintln(stdout, result.String())
		}
		return scanner.Err()
	}

	// Remove first two arguments (set1, set2) from args
	var remainingArgs []string
	if len(args) > 2 {
		remainingArgs = args[2:]
	}
	return processInput(remainingArgs, stdin, processFunc)
}
