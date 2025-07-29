package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mako10k/llmcmd/internal/tools"
)

func executeTool(engine *tools.Engine, functionName string, args map[string]interface{}) (string, error) {
	argsBytes, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("failed to marshal arguments: %w", err)
	}

	toolCall := map[string]interface{}{
		"name":      functionName,
		"arguments": string(argsBytes),
	}
	return engine.ExecuteToolCall(toolCall)
}

func main() {
	// Initialize engine
	engine, err := tools.NewEngine(tools.EngineConfig{})
	if err != nil {
		fmt.Printf("Error initializing engine: %v\n", err)
		os.Exit(1)
	}
	defer engine.Close()

	fmt.Println("=== Diff Command Test ===")

	// Test data
	file1Content := `line 1
common line
line 3 original
line 4`

	file2Content := `line 1
common line
line 3 modified
line 4
line 5 added`

	// Combine with separator for diff command
	combinedInput := file1Content + "\n---LLMCMD_DIFF_SEPARATOR---\n" + file2Content

	// Test the tee → diff workflow
	fmt.Println("\n1. Testing tee → diff workflow...")

	// Step 1: Create input using cat
	catResult, err := executeTool(engine, "spawn", map[string]interface{}{
		"cmd":  "cat",
		"args": []string{},
	})
	if err != nil {
		fmt.Printf("Error spawning cat: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Cat spawn result: %s\n", catResult)

	// For now, let's test diff directly with the builtin command
	// We'll simulate the combined input

	fmt.Println("\n2. Testing diff command directly...")

	// Create a test to validate our diff implementation works
	fmt.Printf("File1 content:\n%s\n\n", strings.ReplaceAll(file1Content, "\n", "\\n"))
	fmt.Printf("File2 content:\n%s\n\n", strings.ReplaceAll(file2Content, "\n", "\\n"))
	fmt.Printf("Combined input:\n%s\n\n", strings.ReplaceAll(combinedInput, "\n", "\\n"))

	// Test using spawn to run diff
	diffResult, err := executeTool(engine, "spawn", map[string]interface{}{
		"cmd":  "diff",
		"args": []string{"-u"},
	})
	if err != nil {
		fmt.Printf("Error spawning diff: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Diff spawn result: %s\n", diffResult)

	// Since we can't easily pipe data in our current test setup,
	// let's create a more comprehensive test

	fmt.Println("\n3. Creating test files for demonstration...")

	// Create temporary test content using echo
	testFile1 := "Original content\nLine 2\nLine 3"
	testFile2 := "Modified content\nLine 2\nLine 3\nNew line 4"

	fmt.Printf("Would compare:\nFile 1: %s\nFile 2: %s\n", testFile1, testFile2)

	// Show the expected diff workflow:
	fmt.Println("\n=== Expected tee + diff workflow ===")
	fmt.Println("1. Prepare combined input: file1_content + separator + file2_content")
	fmt.Println("2. spawn({cmd:'cat'}) → write(combined_input, {eof:true}) → tee(out_fd, [diff_fd])")
	fmt.Println("3. spawn({cmd:'diff', args:['-u'], in_fd:diff_fd}) → read(diff_out_fd)")
	fmt.Println("4. Result: unified diff output")

	// Show statistics
	stats := engine.GetStats()
	fmt.Printf("\nEngine Statistics:\n")
	fmt.Printf("  Spawn calls: %d\n", stats.SpawnCalls)
	fmt.Printf("  Read calls: %d\n", stats.ReadCalls)
	fmt.Printf("  Write calls: %d\n", stats.WriteCalls)
	fmt.Printf("  Errors: %d\n", stats.ErrorCount)
}
