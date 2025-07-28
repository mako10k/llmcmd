package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Common errors for control flow
var (
	ErrShowHelp    = errors.New("show help")
	ErrShowVersion = errors.New("show version")
)

// Config holds all configuration for the application
type Config struct {
	// Command line options
	Prompt      string   // -p: LLM prompt/instructions
	InputFiles  []string // -i: Input file paths (can be specified multiple times)
	OutputFile  string   // -o: Output file path
	Verbose     bool     // -v: Verbose logging
	ConfigFile  string   // -c: Configuration file path

	// Positional arguments
	Instructions string // Remaining arguments as instructions

	// Derived configuration
	ConfigDir string // Directory containing config file
}

// ParseArgs parses command line arguments and returns configuration
func ParseArgs(args []string) (*Config, error) {
	var config Config
	var inputFiles arrayFlags

	// Create a custom FlagSet to handle our specific requirements
	fs := flag.NewFlagSet("llmcmd", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	
	// Define flags
	fs.StringVar(&config.Prompt, "p", "", "LLM prompt/instructions")
	fs.Var(&inputFiles, "i", "Input file path (can be specified multiple times)")
	fs.StringVar(&config.OutputFile, "o", "", "Output file path")
	fs.BoolVar(&config.Verbose, "v", false, "Enable verbose logging")
	fs.StringVar(&config.ConfigFile, "c", "", "Configuration file path")
	
	// Handle help and version flags
	var showHelp, showVersion bool
	fs.BoolVar(&showHelp, "h", false, "Show help")
	fs.BoolVar(&showHelp, "help", false, "Show help")
	fs.BoolVar(&showVersion, "V", false, "Show version")
	fs.BoolVar(&showVersion, "version", false, "Show version")

	// Parse arguments
	err := fs.Parse(args)
	if err != nil {
		return nil, err
	}

	// Handle help/version first
	if showHelp {
		return nil, ErrShowHelp
	}
	if showVersion {
		return nil, ErrShowVersion
	}

	// Copy input files from the custom type
	config.InputFiles = []string(inputFiles)

	// Remaining arguments become instructions
	remaining := fs.Args()
	if len(remaining) > 0 {
		config.Instructions = strings.Join(remaining, " ")
	}

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return nil, err
	}

	// Set default config file if not specified
	if config.ConfigFile == "" {
		if home, err := os.UserHomeDir(); err == nil {
			config.ConfigFile = filepath.Join(home, ".llmcmdrc")
		}
	}

	// Set config directory
	if config.ConfigFile != "" {
		config.ConfigDir = filepath.Dir(config.ConfigFile)
	}

	return &config, nil
}

// validateConfig validates the parsed configuration
func validateConfig(config *Config) error {
	// Either prompt (-p) or instructions must be provided
	if config.Prompt == "" && config.Instructions == "" {
		return fmt.Errorf("either -p (prompt) option or instructions argument must be provided")
	}

	// If both are provided, that's also fine - they will be combined

	// Validate input files exist
	for _, inputFile := range config.InputFiles {
		if _, err := os.Stat(inputFile); os.IsNotExist(err) {
			return fmt.Errorf("input file does not exist: %s", inputFile)
		}
	}

	// Validate output file directory exists if specified
	if config.OutputFile != "" {
		dir := filepath.Dir(config.OutputFile)
		if dir != "." {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				return fmt.Errorf("output directory does not exist: %s", dir)
			}
		}
	}

	return nil
}

// arrayFlags implements flag.Value interface for string arrays
type arrayFlags []string

func (af *arrayFlags) String() string {
	return strings.Join(*af, ",")
}

func (af *arrayFlags) Set(value string) error {
	*af = append(*af, value)
	return nil
}

// ShowHelp displays help information
func ShowHelp() {
	fmt.Print(`llmcmd - LLM Command Line Tool

USAGE:
    llmcmd [OPTIONS] [INSTRUCTIONS]

OPTIONS:
    -p <prompt>         LLM prompt/instructions
    -i <file>           Input file path (can be specified multiple times)
    -o <file>           Output file path  
    -c <file>           Configuration file path (default: ~/.llmcmdrc)
    -v                  Enable verbose logging
    -h, --help          Show this help message
    -V, --version       Show version information

ARGUMENTS:
    INSTRUCTIONS        Command instructions for the LLM

EXAMPLES:
    llmcmd -p "Summarize this file" -i input.txt -o summary.txt
    llmcmd -i file1.txt -i file2.txt "Compare these files"
    llmcmd "Process this text and extract key points" < input.txt

CONFIGURATION:
    Configuration can be provided via:
    1. Command line options (highest priority)
    2. Configuration file (~/.llmcmdrc by default)
    3. Environment variables (lowest priority)

For more information, visit: https://github.com/mako10k/llmcmd
`)
}
