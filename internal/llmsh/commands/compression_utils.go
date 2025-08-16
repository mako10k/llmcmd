package commands

import (
	"io"
	"strings"
)

// CompressionArgs represents parsed compression command arguments
type CompressionArgs struct {
	Decompress bool
}

// ParseCompressionArgs parses common compression arguments
func ParseCompressionArgs(args []string) CompressionArgs {
	result := CompressionArgs{}

	for _, arg := range args {
		if arg == "-d" || arg == "--decompress" {
			result.Decompress = true
		}
	}

	return result
}

// ReadInput reads all input from stdin
func ReadInput(stdin io.ReadWriteCloser) ([]byte, error) {
	return io.ReadAll(stdin)
}

// CheckPrefix checks if text has the expected prefix and returns the content without prefix
func CheckPrefix(text, prefix string) (string, bool) {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, prefix) {
		return trimmed[len(prefix):], true
	}
	return "", false
}
