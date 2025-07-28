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

// Nl numbers lines
func Nl(args []string, stdin io.Reader, stdout io.Writer) error {
	numberNonEmpty := false
	
	// Parse flags
	for _, arg := range args {
		switch arg {
		case "-b":
			numberNonEmpty = true
		}
	}

	scanner := bufio.NewScanner(stdin)
	lineNum := 1
	for scanner.Scan() {
		line := scanner.Text()
		
		if numberNonEmpty && strings.TrimSpace(line) == "" {
			fmt.Fprintln(stdout, line)
		} else {
			fmt.Fprintf(stdout, "%6d\t%s\n", lineNum, line)
			lineNum++
		}
	}

	return scanner.Err()
}

// Tee writes input to both stdout and multiple files
func Tee(args []string, stdin io.Reader, stdout io.Writer) error {
	// For security, we only support writing to stdout
	// File writing should be handled by the main write tool
	
	scanner := bufio.NewScanner(stdin)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintln(stdout, line)
	}

	return scanner.Err()
}

// Rev reverses each line
func Rev(args []string, stdin io.Reader, stdout io.Writer) error {
	scanner := bufio.NewScanner(stdin)
	for scanner.Scan() {
		line := scanner.Text()
		runes := []rune(line)
		
		// Reverse the runes
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		
		fmt.Fprintln(stdout, string(runes))
	}

	return scanner.Err()
}
