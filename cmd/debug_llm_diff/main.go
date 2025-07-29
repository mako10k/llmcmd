package main

import (
	"fmt"
	"os"

	"github.com/mako10k/llmcmd/internal/app"
	"github.com/mako10k/llmcmd/internal/cli"
)

func main() {
	fmt.Println("=== Debug LLM integration for diff command ===")

	// Create test input file
	testInput := `line1
line2
line3
---LLMCMD_DIFF_SEPARATOR---
line1
modified_line2
line3
line4`

	// Write to temp file
	inputFile := "/tmp/test_input.txt"
	outputFile := "/tmp/test_output.txt"

	err := os.WriteFile(inputFile, []byte(testInput), 0644)
	if err != nil {
		fmt.Printf("Error writing input file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Input file content:\n%s\n\n", testInput)

	// Simulate command line arguments for diff
	args := []string{"llmcmd", "--input", inputFile, "--output", outputFile, "--disable-tools", "diff"}

	// Parse CLI
	config, err := cli.ParseArgs(args[1:])
	if err != nil {
		fmt.Printf("CLI parse error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Parsed config: InputFile=%s, OutputFile=%s, DisableTools=%v, Args=%v\n",
		config.InputFile, config.OutputFile, config.DisableTools, config.Args)

	// Create app instance
	appInstance, err := app.NewApp(config)
	if err != nil {
		fmt.Printf("App creation error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("App created successfully")

	// Run the application
	err = appInstance.Run()
	if err != nil {
		fmt.Printf("App run error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("App run completed")

	// Check output
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		fmt.Printf("Output file does not exist: %s\n", outputFile)
	} else {
		outputData, err := os.ReadFile(outputFile)
		if err != nil {
			fmt.Printf("Error reading output file: %v\n", err)
		} else {
			fmt.Printf("Output file content:\n%s\n", string(outputData))
			fmt.Printf("Output length: %d bytes\n", len(outputData))
		}
	}

	// Cleanup
	os.Remove(inputFile)
	os.Remove(outputFile)
}
