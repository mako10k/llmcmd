package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mako10k/llmcmd/internal/llmsh"
)

func main() {
	// Parse command line arguments with strict validation
	var inputFiles, outputFiles []string
	var script string
	var interactive bool
	var scriptFile string
	var virtual bool

	args := os.Args[1:]
	i := 0
	for i < len(args) {
		arg := args[i]
		switch arg {
		case "-i":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "Error: option %s requires an argument\n", arg)
				os.Exit(1)
			}
			i++
			inputFiles = append(inputFiles, args[i])
		case "-o":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "Error: option %s requires an argument\n", arg)
				os.Exit(1)
			}
			i++
			outputFiles = append(outputFiles, args[i])
		case "--virtual":
			virtual = true
		case "-c":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "Error: option %s requires an argument\n", arg)
				os.Exit(1)
			}
			if script != "" {
				fmt.Fprintf(os.Stderr, "Error: cannot specify both -c option and script file\n")
				os.Exit(1)
			}
			if scriptFile != "" {
				fmt.Fprintf(os.Stderr, "Error: cannot specify both -c option and script file\n")
				os.Exit(1)
			}
			i++
			script = args[i]
		case "--help", "-h":
			printUsage()
			return
		case "--version":
			fmt.Printf("%s version %s\n", llmsh.Name, llmsh.Version)
			return
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "Error: unknown option: %s\n", arg)
				printUsage()
				os.Exit(1)
			}
			// This should be a script file
			if script != "" {
				fmt.Fprintf(os.Stderr, "Error: cannot specify both -c option and script file\n")
				os.Exit(1)
			}
			if scriptFile != "" {
				fmt.Fprintf(os.Stderr, "Error: multiple script files specified: %s and %s\n", scriptFile, arg)
				os.Exit(1)
			}
			scriptFile = arg
		}
		i++
	}

	// Validate mutual exclusivity
	if script != "" && scriptFile != "" {
		fmt.Fprintf(os.Stderr, "Error: cannot specify both -c option and script file\n")
		os.Exit(1)
	}

	// Read script from file if specified
	if scriptFile != "" {
		content, err := os.ReadFile(scriptFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading script file %s: %v\n", scriptFile, err)
			os.Exit(1)
		}
		script = string(content)
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
		InputFiles:   inputFiles,
		OutputFiles:  outputFiles,
		VirtualMode:  virtual,
		Debug:        false,
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
	fmt.Println("  -i <file>     Input file hint (can be specified multiple times)")
	fmt.Println("  -o <file>     Output file hint (can be specified multiple times)")
	fmt.Println("  --virtual     Enable virtual mode (restricted file access)")
	fmt.Println("  -c <script>   Execute script string")
	fmt.Println("  -h, --help    Show this help")
	fmt.Println("  --version     Show version")
	fmt.Println("")
	fmt.Println("Arguments:")
	fmt.Println("  script        Script file to execute (mutually exclusive with -c)")
	fmt.Println("")
	fmt.Println("Note: Options -c and script file are mutually exclusive.")
	fmt.Println("      If neither is specified, enters interactive mode or reads from stdin.")
	fmt.Println("      -i/-o options provide file hints to LLM for file selection.")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Printf("  %s -c 'echo hello | grep ello'\n", os.Args[0])
	fmt.Printf("  %s -i data.txt -o result.txt script.llmsh\n", os.Args[0])
	fmt.Printf("  %s --virtual -c 'cat | sort | uniq'\n", os.Args[0])
	fmt.Printf("  echo 'cat file.txt | grep error' | %s\n", os.Args[0])
	fmt.Printf("  %s script.llmsh\n", os.Args[0])
	fmt.Printf("  %s  # Interactive mode\n", os.Args[0])
}
