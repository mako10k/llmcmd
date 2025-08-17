package builtin

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
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

	// Parse options, programs, and files
	suppressPrint := false
	var programs []string
	var files []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "-n":
			suppressPrint = true
		case "-e":
			if i+1 >= len(args) {
				return fmt.Errorf("sed: -e requires an argument")
			}
			i++
			programs = append(programs, args[i])
		default:
			if strings.HasPrefix(a, "-") {
				// Unknown flag ignored for now
				continue
			}
			if len(programs) == 0 {
				// First non-flag treated as program; rest as files
				programs = append(programs, a)
			} else {
				files = append(files, a)
			}
		}
	}
	if len(programs) == 0 {
		return fmt.Errorf("sed: missing expression")
	}

	// Split programs on unescaped ';'
	var rawCmds []string
	for _, p := range programs {
		rawCmds = append(rawCmds, splitBySemicolon(p)...)
	}

	// Parse commands
	cmds := make([]sedCommand, 0, len(rawCmds))
	for _, rc := range rawCmds {
		rc = strings.TrimSpace(rc)
		if rc == "" { continue }
		sc, err := parseSedCommand(rc)
		if err != nil { return err }
		cmds = append(cmds, sc)
	}
	if len(cmds) == 0 {
		return fmt.Errorf("sed: no valid commands")
	}

	// Processor: read all lines first to support '$'
	process := func(r io.Reader) error {
		var lines []string
		scanner := bufio.NewScanner(r)
		for scanner.Scan() { lines = append(lines, scanner.Text()) }
		if err := scanner.Err(); err != nil { return err }

		inRange := make([]bool, len(cmds))
		for idx, line := range lines {
			final := line
			extraPrints := 0
			for j := range cmds {
				matched, end := matchAddress(&cmds[j], idx+1, len(lines), final, &inRange[j])
				if matched {
					var subbed bool
					final, subbed = applySubstitution(final, cmds[j].re, cmds[j].repl, cmds[j].global)
					if cmds[j].printOnSub && subbed { extraPrints++ }
				}
				if end { inRange[j] = false }
			}
			if suppressPrint {
				for k := 0; k < extraPrints; k++ { fmt.Fprintln(stdout, final) }
			} else {
				fmt.Fprintln(stdout, final)
				for k := 0; k < extraPrints; k++ { fmt.Fprintln(stdout, final) }
			}
		}
		return nil
	}

	if len(files) == 0 { return process(stdin) }
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

// --- Address and multi-command support ---

type addrKind int

const (
	addrNone addrKind = iota
	addrLine
	addrLast
	addrRegex
)

type address struct {
	kind addrKind
	num  int
	re   *regexp.Regexp
}

type sedCommand struct {
	a1, a2 address
	hasA1, hasA2 bool
	negate bool
	re    *regexp.Regexp
	repl  string
	global bool
	printOnSub bool
}

func splitBySemicolon(s string) []string {
	var out []string
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' {
			// Escape only for ';' and '\\'. Otherwise, keep backslash literal.
			if i+1 < len(s) {
				next := s[i+1]
				if next == ';' {
					b.WriteByte(';')
					i++
					continue
				}
				if next == '\\' {
					b.WriteByte('\\')
					i++
					continue
				}
			}
			// keep backslash as-is
			b.WriteByte('\\')
			continue
		}
		if c == ';' {
			out = append(out, strings.TrimSpace(b.String()))
			b.Reset()
			continue
		}
		b.WriteByte(c)
	}
	if b.Len() > 0 { out = append(out, strings.TrimSpace(b.String())) }
	return out
}

func parseSedCommand(s string) (sedCommand, error) {
	var cmd sedCommand
	s = strings.TrimSpace(s)
	// Parse optional addresses
	rest := s
	a1, n1, ok1 := parseAddressPrefix(rest)
	if ok1 {
		cmd.a1 = a1; cmd.hasA1 = true
		rest = rest[n1:]
		if len(rest) > 0 && rest[0] == ',' {
			rest = rest[1:]
			a2, n2, ok2 := parseAddressPrefix(rest)
			if !ok2 { return cmd, fmt.Errorf("sed: invalid range address") }
			cmd.a2 = a2; cmd.hasA2 = true
			rest = rest[n2:]
		}
		rest = strings.TrimSpace(rest)
		if len(rest) > 0 && rest[0] == '!' { cmd.negate = true; rest = strings.TrimSpace(rest[1:]) }
	}
	rest = strings.TrimSpace(rest)
	// Must start with s
	p, r, f, err := parseSubstExpression(rest)
	if err != nil { return cmd, err }
	ignoreCase := strings.Contains(f, "i")
	if ignoreCase { p = "(?i)" + p }
	re, err := regexp.Compile(p)
	if err != nil { return cmd, fmt.Errorf("invalid regex pattern: %s", err) }
	cmd.re = re
	cmd.repl = r
	cmd.global = strings.Contains(f, "g")
	cmd.printOnSub = strings.Contains(f, "p")
	return cmd, nil
}

func parseAddressPrefix(s string) (address, int, bool) {
	s = strings.TrimSpace(s)
	var a address
	if s == "" { return a, 0, false }
	// '$'
	if s[0] == '$' { a.kind = addrLast; return a, 1, true }
	// number
	if c := s[0]; c >= '0' && c <= '9' {
		i := 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' { i++ }
		n, _ := strconv.Atoi(s[:i])
		a.kind = addrLine; a.num = n
		return a, i, true
	}
	// /regex/
	if s[0] == '/' {
		i := 1
		var b strings.Builder
		for i < len(s) {
			ch := s[i]
			if ch == '\\' {
				if i+1 < len(s) { b.WriteByte(s[i+1]); i += 2; continue }
				i++
				continue
			}
			if ch == '/' { i++; break }
			b.WriteByte(ch)
			i++
		}
		pattern := b.String()
		re, err := regexp.Compile(pattern)
		if err != nil { return a, 0, false }
		a.kind = addrRegex; a.re = re
		return a, i, true
	}
	return a, 0, false
}

func matchAddress(cmd *sedCommand, lineNo int, total int, line string, inRange *bool) (matched bool, endRange bool) {
	eval := func(a address) bool {
		switch a.kind {
		case addrLine:
			return lineNo == a.num
		case addrLast:
			return lineNo == total
		case addrRegex:
			return a.re.MatchString(line)
		default:
			return true
		}
	}
	var m bool
	if !cmd.hasA1 && !cmd.hasA2 {
		m = true
	} else if cmd.hasA1 && !cmd.hasA2 {
		m = eval(cmd.a1)
	} else {
		// range
		if !*inRange {
			if eval(cmd.a1) { *inRange = true; m = true } else { m = false }
		} else {
			m = true
		}
		if *inRange && eval(cmd.a2) { endRange = true }
	}
	if cmd.negate { m = !m }
	return m, endRange
}
