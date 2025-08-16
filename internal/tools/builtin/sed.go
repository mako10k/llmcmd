package builtin

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Sed performs basic text substitution (s/pattern/replacement/flags)
func Sed(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("sed: missing expression")
	}
	if handled, _, err := HandleHelp(args, stdout, `sed - Stream editor for basic text substitution

Usage: sed s/pattern/replacement/[flags] [file...]

Flags:
	g                 Replace all occurrences (global)
	i                 Case insensitive matching

Options:
	--help, -h        Show this help message

Examples:
	sed s/old/new/g           Replace all "old" with "new"
	sed s/error/ERROR/i       Case-insensitive replacement
`); handled {
		return err
	}

	expr := args[0]
	if !strings.HasPrefix(expr, "s/") {
		return fmt.Errorf("sed: only s/// substitution supported")
	}

	// Parse s/pattern/replacement/flags
	parts := strings.Split(expr[2:], "/")
	if len(parts) < 2 {
		return fmt.Errorf("sed: invalid substitution format")
	}

	pattern := parts[0]
	replacement := parts[1]
	flags := ""
	if len(parts) > 2 {
		flags = parts[2]
	}

	globalReplace := strings.Contains(flags, "g")
	ignoreCase := strings.Contains(flags, "i")

	// Compile regex
	if ignoreCase {
		pattern = "(?i)" + pattern
	}
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern: %s", err)
	}

	scanner := bufio.NewScanner(stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if globalReplace {
			line = regex.ReplaceAllString(line, replacement)
		} else {
			line = regex.ReplaceAllString(line, replacement)
		}
		fmt.Fprintln(stdout, line)
	}

	return scanner.Err()
}
