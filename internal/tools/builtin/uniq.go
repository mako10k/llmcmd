package builtin

import (
	"bufio"
	"fmt"
	"io"
)

// Uniq removes duplicate consecutive lines
func Uniq(args []string, stdin io.Reader, stdout io.Writer) error {
	countOnly := false
	duplicatesOnly := false
	uniqueOnly := false

	// Parse flags
	for _, arg := range args {
		switch arg {
		case "-c":
			countOnly = true
		case "-d":
			duplicatesOnly = true
		case "-u":
			uniqueOnly = true
		}
	}

	scanner := bufio.NewScanner(stdin)
	var prevLine string
	var count int
	first := true

	outputLine := func(line string, cnt int) {
		if countOnly {
			fmt.Fprintf(stdout, "%4d %s\n", cnt, line)
		} else if duplicatesOnly && cnt > 1 {
			fmt.Fprintln(stdout, line)
		} else if uniqueOnly && cnt == 1 {
			fmt.Fprintln(stdout, line)
		} else if !duplicatesOnly && !uniqueOnly {
			fmt.Fprintln(stdout, line)
		}
	}

	for scanner.Scan() {
		line := scanner.Text()

		if first {
			prevLine = line
			count = 1
			first = false
		} else if line == prevLine {
			count++
		} else {
			outputLine(prevLine, count)
			prevLine = line
			count = 1
		}
	}

	// Output the last line
	if !first {
		outputLine(prevLine, count)
	}

	return scanner.Err()
}
