package builtin

import (
	"io"
)

// Cat copies input to output (like Unix cat)
func Cat(args []string, stdin io.Reader, stdout io.Writer) error {
	return processInput(args, stdin, func(input io.Reader) error {
		_, err := io.Copy(stdout, input)
		return err
	})
}
