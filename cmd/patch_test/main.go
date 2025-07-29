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
	if len(os.Args) < 3 {
		fmt.Println("Usage: patch_test <source_file> <patch_file>")
		os.Exit(1)
	}

	sourceFile := os.Args[1]
	patchFile := os.Args[2]

	// Initialize engine
	engine, err := tools.NewEngine(tools.EngineConfig{})
	if err != nil {
		fmt.Printf("Error initializing engine: %v\n", err)
		os.Exit(1)
	}
	defer engine.Close()

	fmt.Println("=== Patch Application Test ===")
	fmt.Printf("Source file: %s\n", sourceFile)
	fmt.Printf("Patch file: %s\n", patchFile)

	// Test 1: Read original file size
	fmt.Println("\n1. Checking original file...")
	result, err := executeTool(engine, "spawn", map[string]interface{}{
		"cmd":  "wc",
		"args": []string{"-c", sourceFile},
	})
	if err != nil {
		fmt.Printf("Error getting file size: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Spawn result: %s\n", result)

	// Parse the spawn result to check success
	if result != "" {
		fmt.Printf("Command started successfully\n")
		// For now, let's use a simple approach to read output
		// We need to know the output fd from the spawn result
		// Since spawn returns in_fd and out_fd via fmt.Sprintf, let's try reading from a fixed fd
	}

	// Try reading from a few possible file descriptors
	var sizeResult string
	for fd := 3; fd <= 10; fd++ {
		tempResult, err := executeTool(engine, "read", map[string]interface{}{
			"fd": fd,
		})
		if err == nil && tempResult != "" {
			sizeResult = tempResult
			fmt.Printf("Successfully read from fd %d\n", fd)
			break
		}
	}

	if sizeResult == "" {
		fmt.Printf("Could not read size output from any fd\n")
		// Continue with test anyway
		sizeResult = "Unknown size"
	}
	fmt.Printf("Original file size: %s\n", sizeResult)

	// Test 2: Show patch content
	fmt.Println("\n2. Examining patch content...")
	result, err = executeTool(engine, "spawn", map[string]interface{}{
		"cmd":  "head",
		"args": []string{"-20", patchFile},
	})
	if err != nil {
		fmt.Printf("Error reading patch: %v\n", err)
		os.Exit(1)
	}

	var patchFd int
	_, err = fmt.Sscanf(result, "Background command started - output available on fd %d", &patchFd)
	if err != nil {
		fmt.Printf("Error parsing patch spawn result: %v\n", err)
		os.Exit(1)
	}

	patchContent, err := executeTool(engine, "read", map[string]interface{}{
		"fd": patchFd,
	})
	if err != nil {
		fmt.Printf("Error reading patch content: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Patch preview (first 20 lines):\n%s\n", patchContent)

	// Test 3: Apply patch (dry-run first)
	fmt.Println("\n3. Testing patch application (dry-run)...")

	// Create backup
	backupFile := sourceFile + ".backup"
	result, err = executeTool(engine, "spawn", map[string]interface{}{
		"cmd":  "cp",
		"args": []string{sourceFile, backupFile},
	})
	if err != nil {
		fmt.Printf("Error creating backup: %v\n", err)
		os.Exit(1)
	}

	var cpFd int
	_, err = fmt.Sscanf(result, "Background command started - output available on fd %d", &cpFd)
	if err != nil {
		fmt.Printf("Error parsing cp result: %v\n", err)
		os.Exit(1)
	}

	cpResult, err := executeTool(engine, "read", map[string]interface{}{
		"fd": cpFd,
	})
	if err != nil {
		fmt.Printf("Error reading cp output: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Backup created: %s\n", cpResult)

	// Dry-run patch
	result, err = executeTool(engine, "spawn", map[string]interface{}{
		"cmd":  "patch",
		"args": []string{"--dry-run", "-p0", sourceFile},
	})
	if err != nil {
		fmt.Printf("Error starting patch dry-run: %v\n", err)
		os.Exit(1)
	}

	var dryRunInFd, dryRunOutFd int
	_, err = fmt.Sscanf(result, "Background command started - input fd %d, output fd %d", &dryRunInFd, &dryRunOutFd)
	if err != nil {
		fmt.Printf("Error parsing dry-run result: %v\n", err)
		os.Exit(1)
	}

	// Send patch content to patch command
	patchFileContent, err := os.ReadFile(patchFile)
	if err != nil {
		fmt.Printf("Error reading patch file: %v\n", err)
		os.Exit(1)
	}

	_, err = executeTool(engine, "write", map[string]interface{}{
		"fd":   dryRunInFd,
		"data": string(patchFileContent),
		"eof":  true,
	})
	if err != nil {
		fmt.Printf("Error writing patch data: %v\n", err)
		os.Exit(1)
	}

	dryRunResult, err := executeTool(engine, "read", map[string]interface{}{
		"fd": dryRunOutFd,
	})
	if err != nil {
		fmt.Printf("Error reading dry-run output: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Dry-run result:\n%s\n", dryRunResult)

	// Test 4: Apply patch for real
	fmt.Println("\n4. Applying patch...")
	result, err = executeTool(engine, "spawn", map[string]interface{}{
		"cmd":  "patch",
		"args": []string{"-p0", sourceFile},
	})
	if err != nil {
		fmt.Printf("Error starting patch application: %v\n", err)
		os.Exit(1)
	}

	var realInFd, realOutFd int
	_, err = fmt.Sscanf(result, "Background command started - input fd %d, output fd %d", &realInFd, &realOutFd)
	if err != nil {
		fmt.Printf("Error parsing patch result: %v\n", err)
		os.Exit(1)
	}

	// Send patch content
	_, err = executeTool(engine, "write", map[string]interface{}{
		"fd":   realInFd,
		"data": string(patchFileContent),
		"eof":  true,
	})
	if err != nil {
		fmt.Printf("Error writing patch data: %v\n", err)
		os.Exit(1)
	}

	realResult, err := executeTool(engine, "read", map[string]interface{}{
		"fd": realOutFd,
	})
	if err != nil {
		fmt.Printf("Error reading patch output: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Patch application result:\n%s\n", realResult)

	// Test 5: Verify changes
	fmt.Println("\n5. Verifying changes...")
	result, err = executeTool(engine, "spawn", map[string]interface{}{
		"cmd":  "wc",
		"args": []string{"-c", sourceFile},
	})
	if err != nil {
		fmt.Printf("Error checking new file size: %v\n", err)
		os.Exit(1)
	}

	var newSizeFd int
	_, err = fmt.Sscanf(result, "Background command started - output available on fd %d", &newSizeFd)
	if err != nil {
		fmt.Printf("Error parsing new size result: %v\n", err)
		os.Exit(1)
	}

	newSizeResult, err := executeTool(engine, "read", map[string]interface{}{
		"fd": newSizeFd,
	})
	if err != nil {
		fmt.Printf("Error reading new size: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("New file size: %s\n", newSizeResult)

	// Show diff
	result, err = executeTool(engine, "spawn", map[string]interface{}{
		"cmd":  "diff",
		"args": []string{"-u", backupFile, sourceFile},
	})
	if err != nil {
		fmt.Printf("Error starting diff: %v\n", err)
		os.Exit(1)
	}

	var diffFd int
	_, err = fmt.Sscanf(result, "Background command started - output available on fd %d", &diffFd)
	if err != nil {
		fmt.Printf("Error parsing diff result: %v\n", err)
		os.Exit(1)
	}

	diffResult, err := executeTool(engine, "read", map[string]interface{}{
		"fd": diffFd,
	})
	if err != nil {
		fmt.Printf("Error reading diff: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Applied changes (diff):\n%s\n", diffResult)

	fmt.Println("\n=== Test Complete ===")
	fmt.Printf("Backup file available at: %s\n", backupFile)

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
