package builtin

import (
	"io"
)

// Cat copies input to output (like Unix cat)
func Cat(args []string, stdin io.Reader, stdout io.Writer) error {
	// cat simply copies stdin to stdout
	_, err := io.Copy(stdout, stdin)
	return err
}
