package builtin

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Cut extracts selected portions of lines from input
func Cut(args []string, stdin io.Reader, stdout io.Writer) error {
	var fields []int
	var delimiter string = "	" // Default delimiter

	// Parse arguments
	for i, arg := range args {
		if arg == "-f" && i+1 < len(args) {
			// Parse field numbers
			fieldSpec := args[i+1]
			for _, fieldStr := range strings.Split(fieldSpec, ",") {
				if field, err := strconv.Atoi(strings.TrimSpace(fieldStr)); err == nil {
					if field > 0 {
						fields = append(fields, field-1) // Convert to 0-indexed
					}
				}
			}
		} else if arg == "-d" && i+1 < len(args) {
			delimiter = args[i+1]
		}
	}

	if len(fields) == 0 {
		return fmt.Errorf("cut: you must specify a list of fields")
	}

	processFunc := func(input io.Reader) error {
		scanner := bufio.NewScanner(input)
		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.Split(line, delimiter)

			var selected []string
			for _, fieldIndex := range fields {
				if fieldIndex < len(parts) {
					selected = append(selected, parts[fieldIndex])
				}
			}

			fmt.Fprintln(stdout, strings.Join(selected, delimiter))
		}
		return scanner.Err()
	}

	return processInput(args, stdin, processFunc)
}
