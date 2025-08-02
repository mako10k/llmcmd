package main

import (
	"bufio"
	"fmt"
	"io"
)

// Tee writes input to both stdout and multiple files
func Tee(args []string, stdin io.Reader, stdout io.Writer) error {
	// For security, we only support writing to stdout
	// File writing should be handled by the main write tool

	scanner := bufio.NewScanner(stdin)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintln(stdout, line)
	}

	return scanner.Err()
}
