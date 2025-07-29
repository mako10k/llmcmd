package main

import (
	"fmt"
	"os"

	"github.com/mako10k/llmcmd/internal/openai"
)

func main() {
	// Change to the llmcmd directory to access test files
	fmt.Println("Starting test...")
	os.Chdir("/home/mako10k/llmcmd")

	// Test tar.gz file analysis with our improved system message
	fmt.Println("=== Testing tar.gz file analysis ===")
	messages := openai.CreateInitialMessages("", "このファイルは何のファイルですか？", []string{"test.tar.gz"}, "", false)
	fmt.Printf("Generated %d messages for tar.gz analysis\n", len(messages))

	fmt.Println("--- System Message (truncated) ---")
	sysContent := messages[0].Content
	if len(sysContent) > 500 {
		sysContent = sysContent[:500] + "...[truncated]"
	}
	fmt.Printf("%s\n\n", sysContent)

	fmt.Println("--- FD Mapping Message ---")
	fdContent := messages[1].Content
	fmt.Printf("%s\n\n", fdContent)

	fmt.Println("--- User Question ---")
	userContent := messages[2].Content
	fmt.Printf("%s\n\n", userContent)

	fmt.Println("=== Key Analysis Points ===")
	fmt.Println("1. System message now includes 'ANALYSIS APPROACH' guidance")
	fmt.Println("2. FD mapping shows file extension (.tar.gz) and type (archive)")
	fmt.Println("3. User question is about file identification, not content search")
	fmt.Println("4. LLM should now understand to use file metadata, not grep content")
}
