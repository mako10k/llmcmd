package openai

import (
	"fmt"
	"os"
	"testing"
)

func TestStreamAndRegularFileDetection(t *testing.T) {
	// Test with regular files
	testFile := "regular_test.txt"
	content := "This is a regular file for testing."
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create regular test file: %v", err)
	}
	defer os.Remove(testFile)

	// Test with a regular file
	messages := CreateInitialMessages("process files", "", []string{testFile}, "", false)
	if len(messages) >= 2 {
		fmt.Printf("Regular File Test:\n%s\n\n", messages[1].Content)
	}

	// Test with /dev/stdin (stream device)
	messages2 := CreateInitialMessages("process stream", "", []string{"/dev/stdin"}, "", false)
	if len(messages2) >= 2 {
		fmt.Printf("Stream Device Test:\n%s\n\n", messages2[1].Content)
	}

	// Test with /dev/null (special device)
	messages3 := CreateInitialMessages("process device", "", []string{"/dev/null"}, "", false)
	if len(messages3) >= 2 {
		fmt.Printf("Special Device Test:\n%s\n\n", messages3[1].Content)
	}

	// Test with file descriptor device
	messages4 := CreateInitialMessages("process fd", "", []string{"/dev/fd/0"}, "", false)
	if len(messages4) >= 2 {
		fmt.Printf("File Descriptor Test:\n%s\n\n", messages4[1].Content)
	}
}
