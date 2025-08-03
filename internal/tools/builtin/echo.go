package builtin

import (
	"fmt"
	"io"
	"strings"
)

// Echo outputs the specified text
func Echo(args []string, stdin io.Reader, stdout io.Writer) error {
	// Join arguments with spaces and output
	output := strings.Join(args, " ")
	fmt.Fprintln(stdout, output)
	return nil
}
