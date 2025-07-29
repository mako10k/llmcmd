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
		fmt.Println("Usage: simple_test <source_file>")
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

	fmt.Println("=== Simple Spawn Tool Test ===")
	fmt.Printf("Source file: %s\n", sourceFile)

	// Test 1: Get file size using wc
	fmt.Println("\n1. Getting file size with wc...")
	result, err := executeTool(engine, "spawn", map[string]interface{}{
		"cmd":  "wc",
		"args": []string{"-c", sourceFile},
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Spawn result: %s\n", result)

	// Test 2: Try to read output from various file descriptors
	fmt.Println("\n2. Trying to read output...")
	var foundOutput bool
	for fd := 3; fd <= 10; fd++ {
		output, err := executeTool(engine, "read", map[string]interface{}{
			"fd": fd,
		})
		if err == nil && output != "" {
			fmt.Printf("Found output on fd %d: %s\n", fd, output)
			foundOutput = true
			break
		}
	}

	if !foundOutput {
		fmt.Println("No output found on any file descriptor")
	}

	// Test 3: Count lines using wc -l
	fmt.Println("\n3. Counting lines with wc -l...")
	result, err = executeTool(engine, "spawn", map[string]interface{}{
		"cmd":  "wc",
		"args": []string{"-l", sourceFile},
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Spawn result: %s\n", result)

	// Try to read the output
	for fd := 3; fd <= 15; fd++ {
		output, err := executeTool(engine, "read", map[string]interface{}{
			"fd": fd,
		})
		if err == nil && output != "" {
			fmt.Printf("Line count output on fd %d: %s\n", fd, output)
			break
		}
	}

	// Test 4: Show first 10 lines using head
	fmt.Println("\n4. Showing first 10 lines with head...")
	result, err = executeTool(engine, "spawn", map[string]interface{}{
		"cmd":  "head",
		"args": []string{"-10", sourceFile},
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Spawn result: %s\n", result)

	// Try to read the output
	for fd := 3; fd <= 20; fd++ {
		output, err := executeTool(engine, "read", map[string]interface{}{
			"fd": fd,
		})
		if err == nil && output != "" && len(output) > 10 {
			fmt.Printf("Head output on fd %d (first 200 chars): %s...\n", fd, output[:200])
			break
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
