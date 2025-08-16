package builtin

import (
	"io"
)

// Cat copies input to output (like Unix cat)
func Cat(args []string, stdin io.Reader, stdout io.Writer) error {
	if handled, _, err := HandleHelp(args, stdout, `cat - Copy input to output

Usage: cat [file...]

Description:
	Concatenate and display files, or standard input if no files specified.

Options:
	--help, -h        Show this help message

Examples:
	cat file.txt      Display contents of file.txt
	cat a.txt b.txt   Display contents of both files
`); handled {
		return err
	}

	return processInput(args, stdin, func(input io.Reader) error {
		_, err := io.Copy(stdout, input)
		return err
	})
}
