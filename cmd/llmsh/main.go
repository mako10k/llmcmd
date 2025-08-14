package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
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
	vfsFd := -1

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
		case "--vfs-fd":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "Error: option %s requires an argument (fd)\n", arg)
				os.Exit(1)
			}
			i++
			fdStr := args[i]
			fdNum, err := strconv.Atoi(fdStr)
			if err != nil || fdNum < 0 {
				fmt.Fprintf(os.Stderr, "Error: invalid fd for --vfs-fd: %s\n", fdStr)
				os.Exit(1)
			}
			vfsFd = fdNum
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
			if strings.HasPrefix(arg, "--vfs-fd=") {
				fdStr := strings.TrimPrefix(arg, "--vfs-fd=")
				fdNum, err := strconv.Atoi(fdStr)
				if err != nil || fdNum < 0 {
					fmt.Fprintf(os.Stderr, "Error: invalid fd for --vfs-fd: %s\n", fdStr)
					os.Exit(1)
				}
				vfsFd = fdNum
				virtual = true
			} else if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "Error: unknown option: %s\n", arg)
				printUsage()
				os.Exit(1)
			} else {
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
		}
		i++
	}

	// Validate mutual exclusivity
	if script != "" && scriptFile != "" {
		fmt.Fprintf(os.Stderr, "Error: cannot specify both -c option and script file\n")
		os.Exit(1)
	}

	// Validate -i/-o options require --virtual and are exclusive with --vfs-fd
	if !virtual && (len(inputFiles) > 0 || len(outputFiles) > 0) {
		fmt.Fprintf(os.Stderr, "Error: -i and -o options require --virtual flag\n")
		os.Exit(1)
	}
	if vfsFd >= 0 && (len(inputFiles) > 0 || len(outputFiles) > 0) {
		fmt.Fprintf(os.Stderr, "Error: -i/-o cannot be combined with --vfs-fd (mutually exclusive)\n")
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
		InputFiles:  inputFiles,
		OutputFiles: outputFiles,
		VirtualMode: virtual,
		Debug:       false,
		VFSFd:       vfsFd,
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
	fmt.Println("  -i <file>     Input file (requires --virtual)")
	fmt.Println("  -o <file>     Output file (requires --virtual)")
	fmt.Println("  --virtual     Enable virtual mode (restricted file access)")
	fmt.Println("  --vfs-fd N    Reuse existing VFS pipe fd (implies --virtual, mutually exclusive with -i/-o)")
	fmt.Println("  -c <script>   Execute script string")
	fmt.Println("  -h, --help    Show this help")
	fmt.Println("  --version     Show version")
	fmt.Println("")
	fmt.Println("Arguments:")
	fmt.Println("  script        Script file to execute (mutually exclusive with -c)")
	fmt.Println("")
	fmt.Println("Note: Options -c and script file are mutually exclusive.")
	fmt.Println("      If neither is specified, enters interactive mode or reads from stdin.")
	fmt.Println("      -i/-o options require --virtual flag for restricted file access.")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Printf("  %s -c 'echo hello | grep ello'\n", os.Args[0])
	fmt.Printf("  %s --virtual -i input.txt -o output.txt -c 'cat input.txt > output.txt'\n", os.Args[0])
	fmt.Printf("  echo 'cat file.txt | grep error' | %s\n", os.Args[0])
	fmt.Printf("  %s script.llmsh\n", os.Args[0])
	fmt.Printf("  %s  # Interactive mode\n", os.Args[0])
}
