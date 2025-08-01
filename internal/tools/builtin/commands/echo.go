package commands

import (
	"io"
	"strings"
)

// Echo implements the echo command
func Echo(args []string, stdin io.Reader, stdout io.Writer) error {
	// Check for help flag
	if checkForHelp(args, echoHelp, stdout) {
		return nil
	}

	// Remove help flags from args
	args = removeHelpFlags(args)

	output := strings.Join(args, " ")
	if len(output) > 0 {
		_, err := stdout.Write([]byte(output + "\n"))
		return err
	}
	_, err := stdout.Write([]byte("\n"))
	return err
}

// echoHelp returns help text for the echo command
func echoHelp() string {
	return `echo - display a line of text

USAGE:
    echo [text...]

DESCRIPTION:
    Echo writes the specified arguments to standard output, separated by spaces
    and followed by a newline.

OPTIONS:
    --help, -h    Show this help message

EXAMPLES:
    echo "Hello, World!"
    echo Hello World
    echo ""
`
}
