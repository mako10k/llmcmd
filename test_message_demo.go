package main

import (
	"fmt"
	"os"

	"github.com/mako10k/llmcmd/internal/openai"
)

func main() {
	fmt.Println("Starting message demo...")
	defer fmt.Println("Demo completed.")

	// Test with tar.gz file
	messages := openai.CreateInitialMessages("", "このファイルは何のファイルですか？", []string{"test.tar.gz"}, "", false)

	fmt.Println("=== Generated Messages for tar.gz file analysis ===")
	for i, msg := range messages {
		fmt.Printf("\n--- Message %d (Role: %s) ---\n", i+1, msg.Role)
		fmt.Printf("%s\n", msg.Content)
		if i < len(messages)-1 {
			fmt.Println("---")
		}
	}

	if len(messages) == 0 {
		fmt.Println("No messages generated!")
		os.Exit(1)
	}
}
