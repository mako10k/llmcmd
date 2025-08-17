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
	if handled, _, err := HandleHelp(args, stdout, `sed - Stream editor for basic text substitution (subset)

Usage: sed [-n] s<delim>pattern<delim>replacement<delim>[flags] [file...]

Flags:
	g                 Replace all occurrences (global)
	i                 Case insensitive matching
	p                 Print the line if a substitution occurred (with -n prints only those lines)

Options:
	--help, -h        Show this help message
	-n                Suppress automatic printing (use with s///p)

Examples:
	sed s/old/new/g                Replace all "old" with "new"
	sed s#error/error_fixed#i      Case-insensitive with custom delimiter
	sed -n 's/foo/bar/p'           Print only lines where substitution occurred
`); handled {
		return err
	}

	// Parse options and expression/files
	suppressPrint := false
	var expr string
	var files []string
	for _, a := range args {
		if a == "-n" {
			suppressPrint = true
			continue
		}
		if expr == "" {
			expr = a
			continue
		}
		files = append(files, a)
	}
	if expr == "" {
		return fmt.Errorf("sed: missing expression")
	}

	pat, repl, flags, err := parseSubstExpression(expr)
	if err != nil {
		return err
	}
	globalReplace := strings.Contains(flags, "g")
	ignoreCase := strings.Contains(flags, "i")
	printOnSub := strings.Contains(flags, "p")

	// Compile regex
	if ignoreCase {
		pat = "(?i)" + pat
	}
	regex, err := regexp.Compile(pat)
	if err != nil {
		return fmt.Errorf("invalid regex pattern: %s", err)
	}

	// Processor for a single reader
	process := func(r io.Reader) error {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			newLine, substituted := applySubstitution(line, regex, repl, globalReplace)
			if suppressPrint {
				if printOnSub && substituted {
					fmt.Fprintln(stdout, newLine)
				}
			} else {
				fmt.Fprintln(stdout, newLine)
				if printOnSub && substituted {
					// sed prints twice when -n is not set and 'p' flag is used
					fmt.Fprintln(stdout, newLine)
				}
			}
		}
		return scanner.Err()
	}

	// Use files if provided, otherwise stdin
	if len(files) == 0 {
		return process(stdin)
	}
	return processInput(files, stdin, process)
}

// parseSubstExpression parses s<delim>pat<delim>repl<delim>[flags]
func parseSubstExpression(expr string) (string, string, string, error) {
	if !strings.HasPrefix(expr, "s") || len(expr) < 2 {
		return "", "", "", fmt.Errorf("sed: only s/// substitution supported")
	}
	delim := rune(expr[1])
	if delim == '\\' || delim == 0 {
		return "", "", "", fmt.Errorf("sed: invalid delimiter")
	}
	i := 2
	readUntil := func(keepBackslash bool) (string, bool) {
		var b strings.Builder
		for i < len(expr) {
			r := rune(expr[i])
			i++
			if r == '\\' {
				// If escaping the delimiter, consume and write delimiter only
				if i < len(expr) && rune(expr[i]) == delim {
					b.WriteRune(delim)
					i++
					continue
				}
				if keepBackslash {
					b.WriteRune('\\')
					if i < len(expr) {
						b.WriteRune(rune(expr[i]))
						i++
					}
					continue
				}
				// if not keeping backslash, just write next rune if present
				if i < len(expr) {
					b.WriteRune(rune(expr[i]))
					i++
				}
				continue
			}
			if r == delim {
				return b.String(), true
			}
			b.WriteRune(r)
		}
		return b.String(), false
	}
	// For both pattern and replacement, keep backslashes (except for escaping the delimiter)
	pat, ok := readUntil(true)
	if !ok {
		return "", "", "", fmt.Errorf("sed: invalid substitution format")
	}
	repl, ok := readUntil(true)
	if !ok {
		return "", "", "", fmt.Errorf("sed: invalid substitution format")
	}
	// Remaining are flags (up to end)
	flags := expr[i:]
	return pat, repl, flags, nil
}

// applySubstitution applies regex to a line with sed-style replacement semantics
// Supports: first-only or global, '&' for full match, '\n' newline, and backrefs \\1..\\9
func applySubstitution(line string, re *regexp.Regexp, repl string, global bool) (string, bool) {
	// helper: expand sed-style replacement using submatch strings
	expandWithSubs := func(subs []string) string {
		var b strings.Builder
		for i := 0; i < len(repl); i++ {
			c := repl[i]
			if c == '&' {
				if len(subs) > 0 {
					b.WriteString(subs[0])
				}
				continue
			}
			if c == '\\' {
				if i+1 >= len(repl) { // trailing backslash -> literal
					b.WriteByte('\\')
					break
				}
				i++
				d := repl[i]
				switch d {
				case 'n':
					b.WriteByte('\n')
				case '\\', '&':
					b.WriteByte(d)
				default:
					if d >= '1' && d <= '9' {
						gi := int(d - '0')
						if gi < len(subs) {
							b.WriteString(subs[gi])
						}
					} else {
						b.WriteByte(d)
					}
				}
				continue
			}
			b.WriteByte(c)
		}
		return b.String()
	}

	substituted := false
	if !global {
		idx := re.FindStringSubmatchIndex(line)
		if idx == nil {
			return line, false
		}
		subs := re.FindStringSubmatch(line)
		substituted = true
		var b strings.Builder
		b.WriteString(line[:idx[0]])
		b.WriteString(expandWithSubs(subs))
		b.WriteString(line[idx[1]:])
		return b.String(), substituted
	}

	// global replace: iterate progressively
	var out strings.Builder
	pos := 0
	for pos <= len(line) {
		segment := line[pos:]
		idx := re.FindStringSubmatchIndex(segment)
		if idx == nil {
			out.WriteString(segment)
			break
		}
		subs := re.FindStringSubmatch(segment)
		substituted = true
		out.WriteString(segment[:idx[0]])
		out.WriteString(expandWithSubs(subs))
		pos += idx[1]
	}
	return out.String(), substituted
}
