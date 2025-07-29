package openai

import (
	"fmt"
	"os"
	"testing"
)

func TestStandardFDInfo(t *testing.T) {
	// Test standard FD info detection
	
	// Test normal case (no files involved)
	messages := CreateInitialMessages("test", "", []string{}, "", false)
	
	if len(messages) >= 2 {
		fmt.Printf("Standard FD Mapping (normal case):\n%s\n\n", messages[1].Content)
	}
	
	// Test with input files 
	err := os.WriteFile("test_file.txt", []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove("test_file.txt")
	
	messagesWithFiles := CreateInitialMessages("test", "", []string{"test_file.txt"}, "", false)
	
	if len(messagesWithFiles) >= 2 {
		fmt.Printf("Standard FD Mapping (with input files):\n%s\n", messagesWithFiles[1].Content)
	}
}

func TestGetStandardFDInfo(t *testing.T) {
	// Test the getStdFileInfo function directly
	
	// Test stdin, stdout, stderr
	for fd := 0; fd <= 2; fd++ {
		info := getStdFileInfo(fd)
		fmt.Printf("FD %d info: %+v\n", fd, info)
	}
}
