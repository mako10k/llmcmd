package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mako10k/llmcmd/internal/llmsh"
)

func main() {
	// Parse command line arguments
	var inputFile, outputFile string
	var script string
	var interactive bool

	args := os.Args[1:]
	for i, arg := range args {
		switch arg {
		case "-i":
			if i+1 < len(args) {
				inputFile = args[i+1]
			}
		case "-o":
			if i+1 < len(args) {
				outputFile = args[i+1]
			}
		case "-c":
			if i+1 < len(args) {
				script = args[i+1]
			}
		case "--help", "-h":
			printUsage()
			return
		case "--version":
			fmt.Printf("%s version %s\n", llmsh.Name, llmsh.Version)
			return
		default:
			if !strings.HasPrefix(arg, "-") && script == "" {
				// Read script from file
				content, err := os.ReadFile(arg)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error reading script file %s: %v\n", arg, err)
					os.Exit(1)
				}
				script = string(content)
			}
		}
	}

	// If no script provided, check if we should read from stdin or be interactive
	if script == "" {
		stat, err := os.Stdin.Stat()
		if err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
			// Reading from pipe/redirection
			content, err := io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading from stdin: %v\n", err)
				os.Exit(1)
			}
			script = string(content)
		} else {
			// Interactive mode
			interactive = true
		}
	}

	// Create shell configuration
	config := &llmsh.Config{
		InputFile:  inputFile,
		OutputFile: outputFile,
		Debug:      false,
	}

	// Create shell instance
	shell, err := llmsh.NewShell(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating shell: %v\n", err)
		os.Exit(1)
	}

	if interactive {
		// Start interactive mode
		fmt.Printf("Welcome to %s %s\n", llmsh.Name, llmsh.Version)
		fmt.Println("Type 'help' for available commands, 'exit' to quit")

		err = shell.Interactive()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error in interactive mode: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Execute script
		err = shell.Execute(script)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error executing script: %v\n", err)
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Printf("Usage: %s [options] [script]\n\n", os.Args[0])
	fmt.Println("Options:")
	fmt.Println("  -i <file>     Input file (accessible as stdin)")
	fmt.Println("  -o <file>     Output file (accessible as stdout)")
	fmt.Println("  -c <script>   Execute script string")
	fmt.Println("  -h, --help    Show this help")
	fmt.Println("  --version     Show version")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Printf("  %s -c 'echo hello | grep ello'\n", os.Args[0])
	fmt.Printf("  echo 'cat file.txt | grep error' | %s\n", os.Args[0])
	fmt.Printf("  %s script.llmsh\n", os.Args[0])
	fmt.Printf("  %s  # Interactive mode\n", os.Args[0])
}
