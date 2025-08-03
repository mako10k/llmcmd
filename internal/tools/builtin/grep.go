package builtin

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Grep searches for patterns in text (basic regex support)
func Grep(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("grep: missing pattern")
	}

	pattern := args[0]
	invertMatch := false
	ignoreCase := false
	lineNumber := false

	// Parse flags (simplified)
	finalPattern := pattern
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			switch arg {
			case "-v":
				invertMatch = true
			case "-i":
				ignoreCase = true
			case "-n":
				lineNumber = true
			}
		} else {
			finalPattern = arg
			break
		}
	}

	// Compile regex
	if ignoreCase {
		finalPattern = "(?i)" + finalPattern
	}
	regex, err := regexp.Compile(finalPattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern: %s", err)
	}

	scanner := bufio.NewScanner(stdin)
	lineNum := 1
	for scanner.Scan() {
		line := scanner.Text()
		matches := regex.MatchString(line)

		if matches != invertMatch { // XOR logic
			if lineNumber {
				fmt.Fprintf(stdout, "%d:%s\n", lineNum, line)
			} else {
				fmt.Fprintln(stdout, line)
			}
		}
		lineNum++
	}

	return scanner.Err()
}
