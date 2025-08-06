package commands

import (
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

// EncodingCommands contains encoding and decoding commands
type EncodingCommands struct{}

// NewEncodingCommands creates a new EncodingCommands instance
func NewEncodingCommands() *EncodingCommands {
	return &EncodingCommands{}
}

// ExecuteUuencode implements uuencode command
func (e *EncodingCommands) ExecuteUuencode(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	filename := "stdin"
	if len(args) > 0 {
		filename = args[0]
	}

	input, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("uuencode: error reading input: %w", err)
	}

	// Uuencoding header
	_, err = stdout.Write([]byte(fmt.Sprintf("begin 644 %s\n", filename)))
	if err != nil {
		return err
	}

	// Process input in chunks of 45 bytes (which encode to 60 characters + length char)
	for i := 0; i < len(input); i += 45 {
		end := i + 45
		if end > len(input) {
			end = len(input)
		}

		chunk := input[i:end]
		encoded := e.uuencodeChunk(chunk)
		_, err := stdout.Write([]byte(encoded + "\n"))
		if err != nil {
			return err
		}
	}

	// Uuencoding footer
	_, err = stdout.Write([]byte("end\n"))
	return err
}

// ExecuteUudecode implements uudecode command
func (e *EncodingCommands) ExecuteUudecode(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	input, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("uudecode: error reading input: %w", err)
	}

	text := string(input)
	lines := strings.Split(text, "\n")

	if len(lines) < 2 {
		return fmt.Errorf("uudecode: invalid input format")
	}

	// Parse header
	if !strings.HasPrefix(lines[0], "begin ") {
		return fmt.Errorf("uudecode: missing 'begin' line")
	}

	// Process encoded lines
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "end" {
			break
		}
		if line == "" {
			continue
		}

		decoded, err := e.uudecodeChunk(line)
		if err != nil {
			return fmt.Errorf("uudecode: error decoding line %d: %w", i+1, err)
		}

		_, err = stdout.Write(decoded)
		if err != nil {
			return err
		}
	}

	return nil
}

// uuencodeChunk encodes a chunk of bytes using uuencoding
func (e *EncodingCommands) uuencodeChunk(data []byte) string {
	if len(data) == 0 {
		return "`"
	}

	// Length character (space + length)
	result := string(rune(' ' + len(data)))

	// Process 3 bytes at a time
	for i := 0; i < len(data); i += 3 {
		// Get up to 3 bytes
		var b1, b2, b3 byte
		b1 = data[i]
		if i+1 < len(data) {
			b2 = data[i+1]
		}
		if i+2 < len(data) {
			b3 = data[i+2]
		}

		// Convert to 4 characters
		c1 := (b1 >> 2) + ' '
		c2 := ((b1&0x03)<<4 | (b2 >> 4)) + ' '
		c3 := ((b2&0x0F)<<2 | (b3 >> 6)) + ' '
		c4 := (b3 & 0x3F) + ' '

		// Replace space with backtick (uuencoding convention)
		if c1 == ' ' {
			c1 = '`'
		}
		if c2 == ' ' {
			c2 = '`'
		}
		if c3 == ' ' {
			c3 = '`'
		}
		if c4 == ' ' {
			c4 = '`'
		}

		result += string([]byte{c1, c2, c3, c4})
	}

	return result
}

// uudecodeChunk decodes a uuencoded line
func (e *EncodingCommands) uudecodeChunk(line string) ([]byte, error) {
	if len(line) == 0 {
		return nil, nil
	}

	// Get length
	length := int(line[0] - ' ')
	if length < 0 || length > 45 {
		return nil, fmt.Errorf("invalid length: %d", length)
	}

	if length == 0 {
		return nil, nil
	}

	// Decode characters
	encoded := line[1:]
	var result []byte

	for i := 0; i < len(encoded); i += 4 {
		if i+3 >= len(encoded) {
			break
		}

		// Get 4 characters
		c1 := encoded[i]
		c2 := encoded[i+1]
		c3 := encoded[i+2]
		c4 := encoded[i+3]

		// Handle backtick
		if c1 == '`' {
			c1 = ' '
		}
		if c2 == '`' {
			c2 = ' '
		}
		if c3 == '`' {
			c3 = ' '
		}
		if c4 == '`' {
			c4 = ' '
		}

		// Convert to bytes
		b1 := (c1-' ')<<2 | (c2-' ')>>4
		b2 := (c2-' ')<<4 | (c3-' ')>>2
		b3 := (c3-' ')<<6 | (c4 - ' ')

		result = append(result, b1)
		if len(result) < length {
			result = append(result, b2)
		}
		if len(result) < length {
			result = append(result, b3)
		}
	}

	// Trim to specified length
	if len(result) > length {
		result = result[:length]
	}

	return result, nil
}

// ExecuteGzip implements gzip compression (simplified)
func (e *EncodingCommands) ExecuteGzip(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	compArgs := ParseCompressionArgs(args)

	input, err := ReadInput(stdin)
	if err != nil {
		return fmt.Errorf("gzip: error reading input: %w", err)
	}

	if compArgs.Decompress {
		// For simplicity, we'll use base64 as a compression placeholder
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(input)))
		if err != nil {
			return fmt.Errorf("gzip: decompression error: %w", err)
		}
		_, err = stdout.Write(decoded)
		return err
	} else {
		// For simplicity, we'll use base64 as a compression placeholder
		encoded := base64.StdEncoding.EncodeToString(input)
		_, err := stdout.Write([]byte(encoded + "\n"))
		return err
	}
}

// ExecuteGunzip implements gunzip decompression (alias for gzip -d)
func (e *EncodingCommands) ExecuteGunzip(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	return e.ExecuteGzip(append([]string{"-d"}, args...), stdin, stdout)
}

// ExecuteBzip2 implements bzip2 compression (simplified)
func (e *EncodingCommands) ExecuteBzip2(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	compArgs := ParseCompressionArgs(args)

	input, err := ReadInput(stdin)
	if err != nil {
		return fmt.Errorf("bzip2: error reading input: %w", err)
	}

	if compArgs.Decompress {
		// For simplicity, we'll use base64 as a compression placeholder with prefix
		content, hasPrefix := CheckPrefix(string(input), "BZ2:")
		if !hasPrefix {
			return fmt.Errorf("bzip2: not a bzip2 file")
		}
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			return fmt.Errorf("bzip2: decompression error: %w", err)
		}
		_, err = stdout.Write(decoded)
		return err
	} else {
		// For simplicity, we'll use base64 as a compression placeholder with prefix
		encoded := "BZ2:" + base64.StdEncoding.EncodeToString(input)
		_, err := stdout.Write([]byte(encoded + "\n"))
		return err
	}
}

// ExecuteBunzip2 implements bunzip2 decompression (alias for bzip2 -d)
func (e *EncodingCommands) ExecuteBunzip2(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	return e.ExecuteBzip2(append([]string{"-d"}, args...), stdin, stdout)
}

// ExecuteXz implements xz compression (simplified)
func (e *EncodingCommands) ExecuteXz(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	compArgs := ParseCompressionArgs(args)

	input, err := ReadInput(stdin)
	if err != nil {
		return fmt.Errorf("xz: error reading input: %w", err)
	}

	if compArgs.Decompress {
		// For simplicity, we'll use base64 as a compression placeholder with prefix
		content, hasPrefix := CheckPrefix(string(input), "XZ:")
		if !hasPrefix {
			return fmt.Errorf("xz: not an xz file")
		}
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			return fmt.Errorf("xz: decompression error: %w", err)
		}
		_, err = stdout.Write(decoded)
		return err
	} else {
		// For simplicity, we'll use base64 as a compression placeholder with prefix
		encoded := "XZ:" + base64.StdEncoding.EncodeToString(input)
		_, err := stdout.Write([]byte(encoded + "\n"))
		return err
	}
}

// ExecuteUnxz implements unxz decompression (alias for xz -d)
func (e *EncodingCommands) ExecuteUnxz(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	return e.ExecuteXz(append([]string{"-d"}, args...), stdin, stdout)
}
