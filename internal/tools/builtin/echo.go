package builtin

import (
	"fmt"
	"io"
	"strings"
)

// Echo outputs the specified text
func Echo(args []string, stdin io.Reader, stdout io.Writer) error {
	if handled, _, err := HandleHelp(args, stdout, `echo - Display text

Usage: echo [text...]

Description:
	Display arguments separated by spaces

Options:
	--help, -h        Show this help message

Examples:
	echo hello world          Output: hello world
	echo "quoted text"        Output: quoted text
`); handled {
		return err
	}

	// Join arguments with spaces and output
	output := strings.Join(args, " ")
	fmt.Fprintln(stdout, output)
	return nil
}
