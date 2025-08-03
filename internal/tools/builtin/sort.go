package builtin

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// Sort sorts lines of text
func Sort(args []string, stdin io.Reader, stdout io.Writer) error {
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

	// Read all lines
	var lines []string
	scanner := bufio.NewScanner(stdin)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Remove duplicates if unique flag is set
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
			a, errA := strconv.ParseFloat(strings.TrimSpace(lines[i]), 64)
			b, errB := strconv.ParseFloat(strings.TrimSpace(lines[j]), 64)

			if errA != nil && errB != nil {
				// Both are not numbers, sort lexically
				result := lines[i] < lines[j]
				return result != reverse
			}
			if errA != nil {
				// a is not a number, b is
				return reverse
			}
			if errB != nil {
				// b is not a number, a is
				return !reverse
			}
			// Both are numbers
			result := a < b
			return result != reverse
		})
	} else {
		sort.Slice(lines, func(i, j int) bool {
			result := lines[i] < lines[j]
			return result != reverse
		})
	}

	// Output sorted lines
	for _, line := range lines {
		fmt.Fprintln(stdout, line)
	}

	return nil
}
