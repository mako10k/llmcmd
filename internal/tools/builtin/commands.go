package builtin

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// CommandFunc represents a built-in command function
type CommandFunc func(args []string, stdin io.Reader, stdout io.Writer) error

// Commands maps command names to their implementations
var Commands = map[string]CommandFunc{
	"cat":        Cat,
	"grep":       Grep,
	"sed":        Sed,
	"head":       Head,
	"tail":       Tail,
	"sort":       Sort,
	"wc":         Wc,
	"tr":         Tr,
	"cut":        Cut,
	"uniq":       Uniq,
	"nl":         Nl,
	"tee":        Tee,
	"rev":        Rev,
	"diff":       Diff,
	"patch":      Patch,
	"get_usages": GetUsages,
}

// compileRegex compiles a regex pattern and returns an error if invalid
func compileRegex(pattern string, ignoreCase bool) (*regexp.Regexp, error) {
	if ignoreCase {
		pattern = "(?i)" + pattern
	}
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %s", err)
	}
	return compiled, nil
}

// appendCount appends formatted count to output slice if condition is true
func appendCount(output []string, count int, condition bool) []string {
	if condition {
		return append(output, fmt.Sprintf("%d", count))
	}
	return output
}

// Cat copies input to output (like Unix cat)
func Cat(args []string, stdin io.Reader, stdout io.Writer) error {
	// cat simply copies stdin to stdout
	_, err := io.Copy(stdout, stdin)
	return err
}

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

	// Compile regex using common function
	regex, err := compileRegex(finalPattern, ignoreCase)
	if err != nil {
		return err
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

// Sed performs basic text substitution (s/pattern/replacement/flags)
func Sed(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("sed: missing expression")
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

	// Compile regex using common function
	regex, err := compileRegex(pattern, ignoreCase)
	if err != nil {
		return err
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

// Head outputs the first n lines (default 10)
func Head(args []string, stdin io.Reader, stdout io.Writer) error {
	n := 10
	if len(args) > 0 && strings.HasPrefix(args[0], "-") {
		if val, err := strconv.Atoi(args[0][1:]); err == nil {
			n = val
		}
	}

	scanner := bufio.NewScanner(stdin)
	count := 0
	for scanner.Scan() && count < n {
		fmt.Fprintln(stdout, scanner.Text())
		count++
	}

	return scanner.Err()
}

// Tail outputs the last n lines (default 10)
func Tail(args []string, stdin io.Reader, stdout io.Writer) error {
	n := 10
	if len(args) > 0 && strings.HasPrefix(args[0], "-") {
		if val, err := strconv.Atoi(args[0][1:]); err == nil {
			n = val
		}
	}

	// Read all lines into memory
	var lines []string
	scanner := bufio.NewScanner(stdin)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Output last n lines
	start := len(lines) - n
	if start < 0 {
		start = 0
	}

	for i := start; i < len(lines); i++ {
		fmt.Fprintln(stdout, lines[i])
	}

	return nil
}

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

// Wc counts lines, words, and characters
func Wc(args []string, stdin io.Reader, stdout io.Writer) error {
	lines := 0
	words := 0
	chars := 0
	bytes := 0

	showLines := true
	showWords := true
	showChars := true
	showBytes := false

	// Parse flags
	flagCount := 0
	for _, arg := range args {
		switch arg {
		case "-l":
			if flagCount == 0 {
				showWords, showChars = false, false
			}
			showLines = true
			flagCount++
		case "-w":
			if flagCount == 0 {
				showLines, showChars = false, false
			}
			showWords = true
			flagCount++
		case "-c":
			if flagCount == 0 {
				showLines, showWords = false, false
			}
			showChars = true
			flagCount++
		case "-m":
			if flagCount == 0 {
				showLines, showWords = false, false
			}
			showBytes = true
			flagCount++
		}
	}

	scanner := bufio.NewScanner(stdin)
	for scanner.Scan() {
		line := scanner.Text()
		lines++
		chars += len([]rune(line)) + 1 // +1 for newline
		bytes += len(line) + 1

		// Count words
		wordScanner := bufio.NewScanner(strings.NewReader(line))
		wordScanner.Split(bufio.ScanWords)
		for wordScanner.Scan() {
			words++
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Output counts
	var output []string
	output = appendCount(output, lines, showLines)
	output = appendCount(output, words, showWords)
	output = appendCount(output, bytes, showBytes)
	output = appendCount(output, chars, showChars && !showBytes)

	fmt.Fprintln(stdout, strings.Join(output, " "))
	return nil
}

// Tr translates or deletes characters
func Tr(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) < 1 {
		return fmt.Errorf("tr: missing operand")
	}

	delete := false
	if args[0] == "-d" {
		delete = true
		args = args[1:]
		if len(args) < 1 {
			return fmt.Errorf("tr: missing character set")
		}
	}

	if delete {
		// Delete characters mode
		deleteSet := args[0]
		deleteRunes := make(map[rune]bool)
		for _, r := range deleteSet {
			deleteRunes[r] = true
		}

		scanner := bufio.NewScanner(stdin)
		for scanner.Scan() {
			line := scanner.Text()
			var result strings.Builder
			for _, r := range line {
				if !deleteRunes[r] {
					result.WriteRune(r)
				}
			}
			fmt.Fprintln(stdout, result.String())
		}
		return scanner.Err()
	}

	// Translation mode
	if len(args) < 2 {
		return fmt.Errorf("tr: missing replacement set")
	}

	fromSet := args[0]
	toSet := args[1]

	fromRunes := []rune(fromSet)
	toRunes := []rune(toSet)

	// Create translation map
	translation := make(map[rune]rune)
	for i, r := range fromRunes {
		if i < len(toRunes) {
			translation[r] = toRunes[i]
		} else if len(toRunes) > 0 {
			// Use last character for remaining translations
			translation[r] = toRunes[len(toRunes)-1]
		}
	}

	scanner := bufio.NewScanner(stdin)
	for scanner.Scan() {
		line := scanner.Text()
		var result strings.Builder
		for _, r := range line {
			if newR, exists := translation[r]; exists {
				result.WriteRune(newR)
			} else {
				result.WriteRune(r)
			}
		}
		fmt.Fprintln(stdout, result.String())
	}

	return scanner.Err()
}
