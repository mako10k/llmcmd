package utils

import (
	"fmt"
	"strconv"
)

// ParseLineCountArgument parses the -n flag from command arguments and returns the line count and remaining args
// Returns (lineCount, remainingArgs, error)
func ParseLineCountArgument(args []string, defaultLines int) (int, []string, error) {
	lines := defaultLines

	// Parse number of lines from arguments
	for i, arg := range args {
		if arg == "-n" && i+1 < len(args) {
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				return 0, nil, fmt.Errorf("invalid number: %s", args[i+1])
			}
			if n < 0 {
				return 0, nil, fmt.Errorf("negative line count: %d", n)
			}
			lines = n
			// Remove processed arguments
			remainingArgs := append(args[:i], args[i+2:]...)
			return lines, remainingArgs, nil
		}
	}

	return lines, args, nil
}
