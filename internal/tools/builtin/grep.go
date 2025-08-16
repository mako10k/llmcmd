package builtin

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Grep searches for patterns in text (basic regex support)
func Grep(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("grep: missing pattern")
	}
	if handled, _, err := HandleHelp(args, stdout, `grep - Search text patterns

Usage: grep [options] pattern [file...]

Options:
	-v                Invert match (show non-matching lines)
	-i                Case insensitive matching
	-n                Show line numbers
	--help, -h        Show this help message

Examples:
	grep "error" log.txt      Find lines containing "error"
	grep -i "warning" file    Case-insensitive search
	grep -v "debug" log       Show lines not containing "debug"
`); handled {
		return err
	}

	// Parse flags and pattern
	invertMatch := false
	ignoreCase := false
	lineNumber := false
	var pattern string
	var files []string

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
		} else if pattern == "" {
			pattern = arg
		} else {
			files = append(files, arg)
		}
	}

	if pattern == "" {
		return fmt.Errorf("grep: missing pattern")
	}

	// Compile regex using common function
	regex, err := compileRegex(pattern, ignoreCase)
	if err != nil {
		return err
	}

	// Process function for each input
	processFunc := func(input io.Reader) error {
		scanner := bufio.NewScanner(input)
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

	// Use files if specified, otherwise stdin
	if len(files) == 0 {
		return processFunc(stdin)
	}

	for _, filename := range files {
		file, err := openFileForReading(filename)
		if err != nil {
			return fmt.Errorf("cannot open %s: %v", filename, err)
		}
		err = processFunc(file)
		file.Close()
		if err != nil {
			return err
		}
	}

	return nil
}
