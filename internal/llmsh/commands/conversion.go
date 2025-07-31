package commands

import (
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ConversionCommands contains data conversion and encoding commands
type ConversionCommands struct{}

// NewConversionCommands creates a new ConversionCommands instance
func NewConversionCommands() *ConversionCommands {
	return &ConversionCommands{}
}

// ExecuteBase64 implements base64 encoding/decoding
func (c *ConversionCommands) ExecuteBase64(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	decode := false

	// Parse arguments
	for _, arg := range args {
		if arg == "-d" || arg == "--decode" {
			decode = true
		}
	}

	input, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("base64: error reading input: %w", err)
	}

	if decode {
		// Decode base64
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(input)))
		if err != nil {
			return fmt.Errorf("base64: decode error: %w", err)
		}
		_, err = stdout.Write(decoded)
		return err
	} else {
		// Encode to base64
		encoded := base64.StdEncoding.EncodeToString(input)
		_, err := stdout.Write([]byte(encoded + "\n"))
		return err
	}
}

// ExecuteOd implements od command (octal dump)
func (c *ConversionCommands) ExecuteOd(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	input, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("od: error reading input: %w", err)
	}

	// Simple octal dump implementation
	offset := 0
	for i := 0; i < len(input); i += 16 {
		// Print offset
		_, err := stdout.Write([]byte(fmt.Sprintf("%07o ", offset)))
		if err != nil {
			return err
		}

		// Print octal values
		for j := 0; j < 16 && i+j < len(input); j += 2 {
			if i+j+1 < len(input) {
				value := uint16(input[i+j]) | (uint16(input[i+j+1]) << 8)
				_, err := stdout.Write([]byte(fmt.Sprintf(" %06o", value)))
				if err != nil {
					return err
				}
			} else {
				value := uint16(input[i+j])
				_, err := stdout.Write([]byte(fmt.Sprintf(" %06o", value)))
				if err != nil {
					return err
				}
			}
		}

		_, err = stdout.Write([]byte("\n"))
		if err != nil {
			return err
		}
		offset += 16
	}

	return nil
}

// ExecuteHexdump implements hexdump command
func (c *ConversionCommands) ExecuteHexdump(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	input, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("hexdump: error reading input: %w", err)
	}

	// Simple hex dump implementation
	for i := 0; i < len(input); i += 16 {
		// Print offset
		_, err := stdout.Write([]byte(fmt.Sprintf("%08x  ", i)))
		if err != nil {
			return err
		}

		// Print hex values
		for j := 0; j < 16; j++ {
			if i+j < len(input) {
				_, err := stdout.Write([]byte(fmt.Sprintf("%02x ", input[i+j])))
				if err != nil {
					return err
				}
			} else {
				_, err := stdout.Write([]byte("   "))
				if err != nil {
					return err
				}
			}
			if j == 7 {
				_, err := stdout.Write([]byte(" "))
				if err != nil {
					return err
				}
			}
		}

		// Print ASCII representation
		_, err = stdout.Write([]byte(" |"))
		if err != nil {
			return err
		}
		for j := 0; j < 16 && i+j < len(input); j++ {
			ch := input[i+j]
			if ch >= 32 && ch <= 126 {
				_, err := stdout.Write([]byte{ch})
				if err != nil {
					return err
				}
			} else {
				_, err := stdout.Write([]byte("."))
				if err != nil {
					return err
				}
			}
		}
		_, err = stdout.Write([]byte("|\n"))
		if err != nil {
			return err
		}
	}

	return nil
}

// ExecuteFmt implements fmt command (text formatting)
func (c *ConversionCommands) ExecuteFmt(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	width := 75

	// Parse width argument
	for i, arg := range args {
		if arg == "-w" && i+1 < len(args) {
			if w, err := strconv.Atoi(args[i+1]); err == nil {
				width = w
			}
		}
	}

	input, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("fmt: error reading input: %w", err)
	}

	text := string(input)
	words := strings.Fields(text)

	if len(words) == 0 {
		return nil
	}

	line := words[0]
	for _, word := range words[1:] {
		if len(line)+1+len(word) <= width {
			line += " " + word
		} else {
			_, err := stdout.Write([]byte(line + "\n"))
			if err != nil {
				return err
			}
			line = word
		}
	}

	if len(line) > 0 {
		_, err := stdout.Write([]byte(line + "\n"))
		if err != nil {
			return err
		}
	}

	return nil
}

// ExecuteFold implements fold command (line wrapping)
func (c *ConversionCommands) ExecuteFold(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	width := 80

	// Parse width argument
	for i, arg := range args {
		if arg == "-w" && i+1 < len(args) {
			if w, err := strconv.Atoi(args[i+1]); err == nil {
				width = w
			}
		}
	}

	// Read input
	input, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("fold: error reading input: %w", err)
	}

	text := string(input)
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		for len(line) > width {
			_, err := stdout.Write([]byte(line[:width] + "\n"))
			if err != nil {
				return err
			}
			line = line[width:]
		}
		if len(line) > 0 {
			_, err := stdout.Write([]byte(line + "\n"))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// ExecuteExpand implements expand command (convert tabs to spaces)
func (c *ConversionCommands) ExecuteExpand(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	tabWidth := 8

	// Parse tab width argument
	for i, arg := range args {
		if (arg == "-t" || arg == "--tabs") && i+1 < len(args) {
			if w, err := strconv.Atoi(args[i+1]); err == nil {
				tabWidth = w
			}
		}
	}

	input, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("expand: error reading input: %w", err)
	}

	text := string(input)
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		col := 0
		result := ""
		for _, ch := range line {
			if ch == '\t' {
				spaces := tabWidth - (col % tabWidth)
				result += strings.Repeat(" ", spaces)
				col += spaces
			} else {
				result += string(ch)
				col++
			}
		}
		if i < len(lines)-1 {
			result += "\n"
		}
		_, err := stdout.Write([]byte(result))
		if err != nil {
			return err
		}
	}

	return nil
}

// ExecuteUnexpand implements unexpand command (convert spaces to tabs)
func (c *ConversionCommands) ExecuteUnexpand(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	tabWidth := 8

	// Parse tab width argument
	for i, arg := range args {
		if (arg == "-t" || arg == "--tabs") && i+1 < len(args) {
			if w, err := strconv.Atoi(args[i+1]); err == nil {
				tabWidth = w
			}
		}
	}

	input, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("unexpand: error reading input: %w", err)
	}

	text := string(input)
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		result := ""
		spaces := 0
		col := 0

		for _, ch := range line {
			if ch == ' ' {
				spaces++
				col++
				if col%tabWidth == 0 && spaces >= tabWidth {
					result += "\t"
					spaces = 0
				}
			} else {
				if spaces > 0 {
					result += strings.Repeat(" ", spaces)
					spaces = 0
				}
				result += string(ch)
				col++
			}
		}

		if spaces > 0 {
			result += strings.Repeat(" ", spaces)
		}

		if i < len(lines)-1 {
			result += "\n"
		}
		_, err := stdout.Write([]byte(result))
		if err != nil {
			return err
		}
	}

	return nil
}
