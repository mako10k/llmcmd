package builtin

import (
	"fmt"
	"io"
	"os"
)

// Tee writes input to both stdout and multiple files
func Tee(args []string, stdin io.Reader, stdout io.Writer) error {
		if handled, _, err := HandleHelp(args, stdout, `tee - Write input to stdout and files

Usage: tee [file...]

Description:
	Copy input to standard output and to files

Options:
	--help, -h        Show this help message

Examples:
	tee file.txt              Copy input to stdout and file.txt
	echo "data" | tee out.txt Display and save data to out.txt
`); handled {
				return err
		}

	processFunc := func(input io.Reader) error {
		// Read all input first
		data, err := io.ReadAll(input)
		if err != nil {
			return err
		}

		// Write to stdout
		if _, err := stdout.Write(data); err != nil {
			return err
		}

		// Write to each specified file
		for _, filename := range args {
			if filename == "--help" || filename == "-h" {
				continue
			}

			// Debug print
			fmt.Fprintf(os.Stderr, "DEBUG: tee writing to file: %s\n", filename)

			// Use VFS-aware file opening
			file, err := openFileForWriting(filename)
			if err != nil {
				return fmt.Errorf("cannot create file %s: %v", filename, err)
			}

			_, err = file.Write(data)
			closeErr := file.Close()

			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
		}

		return nil
	}

	return processInput(args, stdin, processFunc)
}
