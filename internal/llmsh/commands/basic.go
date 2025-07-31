package commands

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// BasicCommands contains basic text processing commands
type BasicCommands struct{}

// NewBasicCommands creates a new BasicCommands instance
func NewBasicCommands() *BasicCommands {
	return &BasicCommands{}
}

// ExecuteEcho implements echo command
func (b *BasicCommands) ExecuteEcho(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	output := strings.Join(args, " ")
	if len(output) > 0 {
		_, err := stdout.Write([]byte(output + "\n"))
		return err
	}
	_, err := stdout.Write([]byte("\n"))
	return err
}

// ExecutePrintf implements printf command
func (b *BasicCommands) ExecutePrintf(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	if len(args) == 0 {
		return nil
	}

	format := args[0]
	values := make([]interface{}, len(args)-1)
	for i, arg := range args[1:] {
		values[i] = arg
	}

	output := fmt.Sprintf(format, values...)
	_, err := stdout.Write([]byte(output))
	return err
}

// ExecuteTrue implements true command
func (b *BasicCommands) ExecuteTrue(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	return nil
}

// ExecuteFalse implements false command
func (b *BasicCommands) ExecuteFalse(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	return fmt.Errorf("false")
}

// ExecuteYes implements yes command
func (b *BasicCommands) ExecuteYes(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	output := "y"
	if len(args) > 0 {
		output = strings.Join(args, " ")
	}

	// Output the string repeatedly (limited for safety)
	for i := 0; i < 1000; i++ {
		_, err := stdout.Write([]byte(output + "\n"))
		if err != nil {
			return err
		}
	}
	return nil
}

// ExecuteBasename implements basename command
func (b *BasicCommands) ExecuteBasename(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	if len(args) == 0 {
		return fmt.Errorf("basename: missing operand")
	}

	path := args[0]
	suffix := ""
	if len(args) > 1 {
		suffix = args[1]
	}

	// Get basename
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash >= 0 {
		path = path[lastSlash+1:]
	}

	// Remove suffix if specified
	if suffix != "" && strings.HasSuffix(path, suffix) {
		path = path[:len(path)-len(suffix)]
	}

	_, err := stdout.Write([]byte(path + "\n"))
	return err
}

// ExecuteDirname implements dirname command
func (b *BasicCommands) ExecuteDirname(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	if len(args) == 0 {
		return fmt.Errorf("dirname: missing operand")
	}

	path := args[0]
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash < 0 {
		path = "."
	} else if lastSlash == 0 {
		path = "/"
	} else {
		path = path[:lastSlash]
	}

	_, err := stdout.Write([]byte(path + "\n"))
	return err
}

// ExecuteSeq implements seq command
func (b *BasicCommands) ExecuteSeq(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	if len(args) == 0 {
		return fmt.Errorf("seq: missing operand")
	}

	var start, end, step int = 1, 1, 1
	var err error

	switch len(args) {
	case 1:
		end, err = strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("seq: invalid number: %s", args[0])
		}
	case 2:
		start, err = strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("seq: invalid number: %s", args[0])
		}
		end, err = strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("seq: invalid number: %s", args[1])
		}
	case 3:
		start, err = strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("seq: invalid number: %s", args[0])
		}
		step, err = strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("seq: invalid number: %s", args[1])
		}
		end, err = strconv.Atoi(args[2])
		if err != nil {
			return fmt.Errorf("seq: invalid number: %s", args[2])
		}
	default:
		return fmt.Errorf("seq: too many arguments")
	}

	if step == 0 {
		return fmt.Errorf("seq: step cannot be zero")
	}

	if step > 0 {
		for i := start; i <= end; i += step {
			_, err := stdout.Write([]byte(fmt.Sprintf("%d\n", i)))
			if err != nil {
				return err
			}
		}
	} else {
		for i := start; i >= end; i += step {
			_, err := stdout.Write([]byte(fmt.Sprintf("%d\n", i)))
			if err != nil {
				return err
			}
		}
	}

	return nil
}
