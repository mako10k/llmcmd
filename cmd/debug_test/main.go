package main

import (
	"encoding/json"
	"fmt"
	"os"

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
	if len(os.Args) < 2 {
		fmt.Println("Usage: debug_test <source_file>")
		os.Exit(1)
	}

	sourceFile := os.Args[1]

	// Initialize engine
	engine, err := tools.NewEngine(tools.EngineConfig{})
	if err != nil {
		fmt.Printf("Error initializing engine: %v\n", err)
		os.Exit(1)
	}
	defer engine.Close()

	fmt.Println("=== Debug Spawn Tool Test ===")
	fmt.Printf("Source file: %s\n", sourceFile)

	// Test with cat command (simple output)
	fmt.Println("\n1. Testing with cat (simple command)...")
	result, err := executeTool(engine, "spawn", map[string]interface{}{
		"cmd":  "cat",
		"args": []string{sourceFile},
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Spawn result: %s\n", result)

	// Wait a moment for command to process
	fmt.Println("Waiting for command to process...")
	
	// Check all file descriptors 
	fmt.Println("\n2. Checking all file descriptors...")
	for fd := 0; fd <= 20; fd++ {
		output, err := executeTool(engine, "read", map[string]interface{}{
			"fd": fd,
		})
		if err != nil {
			fmt.Printf("  fd %d: Error - %v\n", fd, err)
		} else if output == "" {
			fmt.Printf("  fd %d: Empty\n", fd)
		} else {
			fmt.Printf("  fd %d: Has content (%d bytes)\n", fd, len(output))
			if len(output) > 100 {
				fmt.Printf("    Preview: %s...\n", output[:100])
			} else {
				fmt.Printf("    Content: %s\n", output)
			}
		}
	}

	// Show statistics
	stats := engine.GetStats()
	fmt.Printf("\nEngine Statistics:\n")
	fmt.Printf("  Spawn calls: %d\n", stats.SpawnCalls)
	fmt.Printf("  Read calls: %d\n", stats.ReadCalls)
	fmt.Printf("  Write calls: %d\n", stats.WriteCalls)
	fmt.Printf("  Bytes read: %d\n", stats.BytesRead)
	fmt.Printf("  Bytes written: %d\n", stats.BytesWritten)
	fmt.Printf("  Errors: %d\n", stats.ErrorCount)
}
