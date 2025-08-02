package main

import (
	"fmt"
	"io"
	"strings"
)

// commandHelp maps command names to their help text
var commandHelp = map[string]string{
	"cat": `cat [file...] - Display file contents
Usage: cat [file1 file2 ...]
  Without files: reads from stdin
  With files: concatenates and displays files`,

	"grep": `grep [options] pattern [file...] - Search for patterns
Usage: grep [-i] [-v] [-n] [-c] pattern [file1 file2 ...]
  -i: ignore case
  -v: invert match (show non-matching lines)
  -n: show line numbers
  -c: count matches only
  Without files: reads from stdin`,

	"sed": `sed [options] 's/pattern/replacement/[flags]' [file...] - Stream editor
Usage: sed [-i] 's/pattern/replacement/[flags]' [file1 file2 ...]
  -i: edit files in-place
  flags: g (global), i (ignore case)
  Without files: reads from stdin`,

	"head": `head [options] [file...] - Display first lines
Usage: head [-n lines] [file1 file2 ...]
  -n: number of lines (default: 10)
  Without files: reads from stdin`,

	"tail": `tail [options] [file...] - Display last lines
Usage: tail [-n lines] [file1 file2 ...]
  -n: number of lines (default: 10)
  Without files: reads from stdin`,

	"sort": `sort [options] [file...] - Sort lines
Usage: sort [-r] [-n] [-u] [file1 file2 ...]
  -r: reverse order
  -n: numeric sort
  -u: unique (remove duplicates)
  Without files: reads from stdin`,

	"wc": `wc [options] [file...] - Count lines, words, characters
Usage: wc [-l] [-w] [-c] [file1 file2 ...]
  -l: count lines
  -w: count words
  -c: count characters
  Without options: shows all counts
  Without files: reads from stdin`,

	"tr": `tr [options] set1 [set2] - Translate characters
Usage: tr [-d] [-s] set1 [set2]
  -d: delete characters in set1
  -s: squeeze repeats
  set1/set2: character sets or ranges
  Always reads from stdin`,

	"cut": `cut [options] [file...] - Extract columns
Usage: cut -d delimiter -f fields [file1 file2 ...]
  -d: field delimiter
  -f: field numbers (1,3 or 1-3)
  Without files: reads from stdin`,

	"uniq": `uniq [options] [file...] - Remove duplicate lines
Usage: uniq [-c] [-d] [-u] [file1 file2 ...]
  -c: count occurrences
  -d: show only duplicate lines
  -u: show only unique lines
  Input must be sorted
  Without files: reads from stdin`,

	"nl": `nl [options] [file...] - Number lines
Usage: nl [-b style] [file1 file2 ...]
  -b a: number all lines
  -b t: number non-empty lines (default)
  Without files: reads from stdin`,

	"tee": `tee [options] file... - Copy input to files and stdout
Usage: tee [-a] file1 [file2 ...]
  -a: append to files
  Always reads from stdin`,

	"rev": `rev [file...] - Reverse lines character by character
Usage: rev [file1 file2 ...]
  Without files: reads from stdin`,

	"diff": `diff [options] file1 file2 - Compare files
Usage: diff [-u] [-c] file1 file2
  -u: unified format
  -c: context format
  Requires exactly two files`,

	"patch": `patch [options] [file] - Apply patch
Usage: patch [-p num] [file]
  -p: strip path components
  Reads patch from stdin`,
}

// Help implements the help command for builtin commands
func Help(args []string, stdin io.Reader, stdout io.Writer) error {
	// If no arguments, show all available commands
	if len(args) == 0 {
		fmt.Fprint(stdout, "AVAILABLE BUILTIN COMMANDS:\n\n")

		// Hardcoded list to avoid initialization cycle
		commands := []string{
			"cat", "cut", "diff", "grep", "head", "nl", "patch",
			"rev", "sed", "sort", "tail", "tee", "tr", "uniq", "wc",
		}

		for _, cmd := range commands {
			if help, exists := commandHelp[cmd]; exists {
				// Extract the brief description (first line)
				lines := strings.Split(help, "\n")
				if len(lines) > 0 {
					fmt.Fprintf(stdout, "  %s\n", lines[0])
				}
			} else {
				fmt.Fprintf(stdout, "  %s - (no help available)\n", cmd)
			}
		}

		fmt.Fprint(stdout, "\nUse 'help <command>' for detailed information about a specific command.\n")
		return nil
	}

	// Show help for specific commands
	for i, cmd := range args {
		if i > 0 {
			fmt.Fprint(stdout, "\n")
		}

		if help, exists := commandHelp[cmd]; exists {
			fmt.Fprint(stdout, help)
			fmt.Fprint(stdout, "\n")
		} else {
			fmt.Fprintf(stdout, "%s - (no detailed help available)\n", cmd)
		}
	}

	return nil
}
