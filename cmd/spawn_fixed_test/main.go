package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mako10k/llmcmd/internal/tools"
)

func main() {
	fmt.Println("=== Fixed Spawn Tool Test ===")

	// Create engine
	config := tools.EngineConfig{
		InputFiles:  []string{},
		OutputFile:  "",
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
		"name": "spawn",
		"arguments": `{"cmd": "diff"}`,
	}
	
	result, err := engine.ExecuteToolCall(spawnCall)
	if err != nil {
		fmt.Printf("Error spawning diff: %v\n", err)
		return
	}
	
	fmt.Printf("Spawn result: %s\n", result)
	
	// Parse spawn result to get file descriptors
	var spawnResult map[string]interface{}
	err = json.Unmarshal([]byte(result[strings.LastIndex(result, "{"):]), &spawnResult)
	if err != nil {
		fmt.Printf("Failed to parse spawn result: %v\n", err)
		return
	}
	
	inFd := int(spawnResult["in_fd"].(float64))
	outFd := int(spawnResult["out_fd"].(float64))
	
	fmt.Printf("Got file descriptors: in_fd=%d, out_fd=%d\n", inFd, outFd)
	
	// Test 2: Write diff input
	fmt.Println("\n2. Writing diff input...")
	
	diffInput := "line 1\ncommon line\nline 3 original\nline 4\n---LLMCMD_DIFF_SEPARATOR---\nline 1\ncommon line\nline 3 modified\nline 4\nline 5 added"
	
	writeCall := map[string]interface{}{
		"name": "write",
		"arguments": fmt.Sprintf(`{"fd": %d, "data": %q, "eof": true}`, inFd, diffInput),
	}
	
	writeResult, err := engine.ExecuteToolCall(writeCall)
	if err != nil {
		fmt.Printf("Error writing to diff: %v\n", err)
		return
	}
	
	fmt.Printf("Write result: %s\n", writeResult)
	
	// Test 3: Read diff output
	fmt.Println("\n3. Reading diff output...")
	
	readCall := map[string]interface{}{
		"name": "read",
		"arguments": fmt.Sprintf(`{"fd": %d}`, outFd),
	}
	
	readResult, err := engine.ExecuteToolCall(readCall)
	if err != nil {
		fmt.Printf("Error reading diff output: %v\n", err)
		return
	}
	
	fmt.Printf("Diff output:\n%s\n", readResult)
	
	// Test 4: spawn patch command
	fmt.Println("\n4. Testing spawn patch command...")
	
	patchSpawnCall := map[string]interface{}{
		"name": "spawn",
		"arguments": `{"cmd": "patch"}`,
	}
	
	patchResult, err := engine.ExecuteToolCall(patchSpawnCall)
	if err != nil {
		fmt.Printf("Error spawning patch: %v\n", err)
		return
	}
	
	fmt.Printf("Patch spawn result: %s\n", patchResult)

	fmt.Println("\n=== Test Complete ===")
	
	// Show engine statistics
	stats := engine.GetStats()
	fmt.Printf("\nEngine Statistics:\n")
	fmt.Printf("  Spawn calls: %d\n", stats.SpawnCalls)
	fmt.Printf("  Read calls: %d\n", stats.ReadCalls) 
	fmt.Printf("  Write calls: %d\n", stats.WriteCalls)
	fmt.Printf("  Errors: %d\n", stats.ErrorCount)
}
