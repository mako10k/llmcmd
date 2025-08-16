package builtin

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
)

// Sort sorts lines of text
func Sort(args []string, stdin io.Reader, stdout io.Writer) error {
	if handled, _, err := HandleHelp(args, stdout, `sort - Sort lines of text

Usage: sort [options] [file...]

Options:
	-r                Reverse sort order
	-n                Numeric sort
	-u                Remove duplicate lines
	--help, -h        Show this help message

Examples:
	sort file.txt             Sort lines alphabetically
	sort -r file.txt          Sort in reverse order
	sort -n numbers.txt       Sort numerically
`); handled {
		return err
	}

	reverse := false
	numeric := false
	unique := false

	// Parse flags
	for _, arg := range args {
		switch arg {
		case "-r":
			reverse = true
		case "-n":
			numeric = true
		case "-u":
			unique = true
		}
	}

	processFunc := func(input io.Reader) error {
		var lines []string
		scanner := bufio.NewScanner(input)

		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			return err
		}

		// Remove duplicates if -u flag is set
		if unique {
			seen := make(map[string]bool)
			var uniqueLines []string
			for _, line := range lines {
				if !seen[line] {
					seen[line] = true
					uniqueLines = append(uniqueLines, line)
				}
			}
			lines = uniqueLines
		}

		// Sort lines
		if numeric {
			sort.Slice(lines, func(i, j int) bool {
				a, errA := strconv.Atoi(lines[i])
				b, errB := strconv.Atoi(lines[j])
				if errA != nil || errB != nil {
					// Fall back to string comparison if not numeric
					if reverse {
						return lines[i] > lines[j]
					}
					return lines[i] < lines[j]
				}
				if reverse {
					return a > b
				}
				return a < b
			})
		} else {
			if reverse {
				sort.Sort(sort.Reverse(sort.StringSlice(lines)))
			} else {
				sort.Strings(lines)
			}
		}

		for _, line := range lines {
			fmt.Fprintln(stdout, line)
		}

		return nil
	}

	return processInput(args, stdin, processFunc)
}
