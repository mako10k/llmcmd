package builtin

import (
	"io"
)

// Man provides manual page functionality by delegating to the help command
func Man(args []string, stdin io.Reader, stdout io.Writer) error {
	// Simply delegate to the help command
	return GetHelp(args, stdin, stdout)
}
