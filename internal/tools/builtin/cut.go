package builtin

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Cut extracts specific fields or character ranges from lines
func Cut(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("cut: missing field specification")
	}

	var fields []int
	var charRange []int
	delimiter := "\t"
	useFields := false
	useChars := false

	// Parse arguments
	for i, arg := range args {
		switch arg {
		case "-f":
			if i+1 < len(args) {
				useFields = true
				fieldSpec := args[i+1]
				for _, spec := range strings.Split(fieldSpec, ",") {
					if field, err := strconv.Atoi(spec); err == nil {
						fields = append(fields, field-1) // Convert to 0-based
					}
				}
			}
		case "-c":
			if i+1 < len(args) {
				useChars = true
				charSpec := args[i+1]
				for _, spec := range strings.Split(charSpec, ",") {
					if char, err := strconv.Atoi(spec); err == nil {
						charRange = append(charRange, char-1) // Convert to 0-based
					}
				}
			}
		case "-d":
			if i+1 < len(args) {
				delimiter = args[i+1]
			}
		}
	}

	scanner := bufio.NewScanner(stdin)
	for scanner.Scan() {
		line := scanner.Text()

		if useFields {
			parts := strings.Split(line, delimiter)
			var selected []string
			for _, fieldIdx := range fields {
				if fieldIdx >= 0 && fieldIdx < len(parts) {
					selected = append(selected, parts[fieldIdx])
				}
			}
			fmt.Fprintln(stdout, strings.Join(selected, delimiter))
		} else if useChars {
			runes := []rune(line)
			var selected []rune
			for _, charIdx := range charRange {
				if charIdx >= 0 && charIdx < len(runes) {
					selected = append(selected, runes[charIdx])
				}
			}
			fmt.Fprintln(stdout, string(selected))
		} else {
			fmt.Fprintln(stdout, line)
		}
	}

	return scanner.Err()
}
