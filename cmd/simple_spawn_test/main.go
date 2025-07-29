package main

import (
	"fmt"
	"strings"

	"github.com/mako10k/llmcmd/internal/tools"
)

func main() {
	fmt.Println("=== Simple Spawn Test for diff/patch ===")

	// Create engine
	config := tools.EngineConfig{
		InputFiles: []string{},
		OutputFile: "",
	}
	engine, err := tools.NewEngine(config)
	if err != nil {
		fmt.Printf("Error creating engine: %v\n", err)
		return
	}
	defer engine.Close()

	// Test 1: spawn diff command
	fmt.Println("\n1. Testing spawn diff command...")

	spawnCall := map[string]interface{}{
		"name":      "spawn",
		"arguments": `{"cmd": "diff"}`,
	}

	result, err := engine.ExecuteToolCall(spawnCall)
	if err != nil {
		fmt.Printf("Error spawning diff: %v\n", err)
		return
	}

	fmt.Printf("Spawn result: %s\n", result)

	// Check if spawn was successful
	if strings.Contains(result, "Background command 'diff' started") {
		fmt.Println("✅ diff command spawn: SUCCESS")
	} else {
		fmt.Println("❌ diff command spawn: FAILED")
	}

	// Test 2: spawn patch command
	fmt.Println("\n2. Testing spawn patch command...")

	patchSpawnCall := map[string]interface{}{
		"name":      "spawn",
		"arguments": `{"cmd": "patch"}`,
	}

	patchResult, err := engine.ExecuteToolCall(patchSpawnCall)
	if err != nil {
		fmt.Printf("Error spawning patch: %v\n", err)
		return
	}

	fmt.Printf("Patch spawn result: %s\n", patchResult)

	// Check if spawn was successful
	if strings.Contains(patchResult, "Background command 'patch' started") {
		fmt.Println("✅ patch command spawn: SUCCESS")
	} else {
		fmt.Println("❌ patch command spawn: FAILED")
	}

	// Test 3: spawn other builtin commands
	fmt.Println("\n3. Testing other builtin commands...")

	commands := []string{"cat", "grep", "wc", "tee", "rev", "cut", "uniq", "nl"}

	for _, cmd := range commands {
		cmdCall := map[string]interface{}{
			"name":      "spawn",
			"arguments": fmt.Sprintf(`{"cmd": "%s"}`, cmd),
		}

		cmdResult, err := engine.ExecuteToolCall(cmdCall)
		if err != nil {
			fmt.Printf("❌ %s: ERROR - %v\n", cmd, err)
		} else if strings.Contains(cmdResult, fmt.Sprintf("Background command '%s' started", cmd)) {
			fmt.Printf("✅ %s: SUCCESS\n", cmd)
		} else {
			fmt.Printf("❌ %s: UNEXPECTED RESULT - %s\n", cmd, cmdResult)
		}
	}

	fmt.Println("\n=== Test Complete ===")

	// Show engine statistics
	stats := engine.GetStats()
	fmt.Printf("\nEngine Statistics:\n")
	fmt.Printf("  Spawn calls: %d\n", stats.SpawnCalls)
	fmt.Printf("  Read calls: %d\n", stats.ReadCalls)
	fmt.Printf("  Write calls: %d\n", stats.WriteCalls)
	fmt.Printf("  Errors: %d\n", stats.ErrorCount)
}
