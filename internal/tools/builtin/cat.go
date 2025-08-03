package builtin

import (
	"fmt"
	"io"
)

// Cat copies input to output (like Unix cat)
func Cat(args []string, stdin io.Reader, stdout io.Writer) error {
	// Check for help option
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			fmt.Fprint(stdout, `cat - Copy input to output

Usage: cat [file...]

Description:
  Concatenate and display files, or standard input if no files specified.

Options:
  --help, -h        Show this help message

Examples:
  cat file.txt      Display contents of file.txt
  cat a.txt b.txt   Display contents of both files
`)
			return nil
		}
	}

	return processInput(args, stdin, func(input io.Reader) error {
		_, err := io.Copy(stdout, input)
		return err
	})
}
