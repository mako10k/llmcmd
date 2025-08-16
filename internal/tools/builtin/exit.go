package builtin

import (
	"fmt"
	"io"
	"os"
	"strconv"
)

// Exit implements the exit command for shell termination
func Exit(args []string, stdin io.Reader, stdout io.Writer) error {
	if handled, _, err := HandleHelp(args, stdout, `exit - Exit the shell

Usage: exit [exit_code]

Description:
	Exit the shell with optional exit code (default: 0)

Parameters:
	exit_code     Integer exit code (0-255, default: 0)

Options:
	--help, -h    Show this help message

Examples:
	exit          Exit with code 0 (success)
	exit 0        Exit with code 0 (success)
	exit 1        Exit with code 1 (error)
`); handled {
		return err
	}

	exitCode := 0

	// Parse exit code if provided
	if len(args) > 0 {
		code, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("exit: invalid exit code '%s': must be a number", args[0])
		}
		if code < 0 || code > 255 {
			return fmt.Errorf("exit: exit code %d out of range (0-255)", code)
		}
		exitCode = code
	}

	// Exit the process
	os.Exit(exitCode)
	return nil // This line should never be reached
}
