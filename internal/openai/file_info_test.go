package openai

import (
	"fmt"
	"os"
	"testing"
)

func TestFileInfoDisplay(t *testing.T) {
	// Test with multiple files of different types and sizes

	// Create test files
	files := map[string][]byte{
		"small.txt":   []byte("This is a small text file with minimal content."),
		"config.json": []byte(`{"name": "test", "version": "1.0", "data": [1,2,3,4,5]}`),
		"medium.log":  make([]byte, 50*1024),      // 50KB
		"large.bin":   make([]byte, 15*1024*1024), // 15MB
	}

	var testFiles []string
	for filename, content := range files {
		if filename == "medium.log" {
			// Fill with log-like content
			logContent := "2024-01-01 12:00:00 INFO Starting application\n"
			for i := 0; i < len(content); i += len(logContent) {
				copy(content[i:], logContent)
			}
		}

		err := os.WriteFile(filename, content, 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
		defer os.Remove(filename)
		testFiles = append(testFiles, filename)
	}

	// Add stream devices to test
	testFiles = append(testFiles, "/dev/stdin", "/dev/fd/3")

	// Test with multiple files
	messages := CreateInitialMessages("analyze these files", "", testFiles, "", false)

	// Print just the second message which contains file descriptor mapping
	if len(messages) >= 2 {
		fmt.Printf("File Descriptor Mapping Message:\n%s\n", messages[1].Content)
	}
}
