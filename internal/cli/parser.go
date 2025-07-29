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
	ErrInstall     = errors.New("install system")
	ErrListPresets = errors.New("list presets")
)

// Config holds all configuration for the application
type Config struct {
	// Command line options
	Prompt       string   // -p: LLM prompt/instructions (free text)
	Preset       string   // -r/--preset: Preset prompt key
	ListPresets  bool     // --list-presets: Show available prompt presets
	InputFiles   []string // -i: Input file paths (can be specified multiple times)
	OutputFile   string   // -o: Output file path
	Verbose      bool     // -v: Verbose logging
	ShowStats    bool     // --stats: Show detailed statistics
	ConfigFile   string   // -c: Configuration file path
	NoStdin       bool     // --no-stdin: Skip reading from stdin

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

	// Define flags with both short and long options where appropriate
	fs.StringVar(&config.Prompt, "p", "", "LLM prompt/instructions (free text)")
	fs.StringVar(&config.Prompt, "prompt", "", "LLM prompt/instructions (free text)")
	
	fs.StringVar(&config.Preset, "r", "", "Use predefined prompt preset (see --list-presets)")
	fs.StringVar(&config.Preset, "preset", "", "Use predefined prompt preset (see --list-presets)")
	fs.BoolVar(&config.ListPresets, "list-presets", false, "List available prompt presets and exit")

	fs.Var(&inputFiles, "i", "Input file path (can be specified multiple times)")
	fs.Var(&inputFiles, "input", "Input file path (can be specified multiple times)")

	fs.StringVar(&config.OutputFile, "o", "", "Output file path")
	fs.StringVar(&config.OutputFile, "output", "", "Output file path")

	fs.StringVar(&config.ConfigFile, "c", "", "Configuration file path")
	fs.StringVar(&config.ConfigFile, "config", "", "Configuration file path")

	fs.BoolVar(&config.Verbose, "v", false, "Enable verbose logging")
	fs.BoolVar(&config.Verbose, "verbose", false, "Enable verbose logging")

	fs.BoolVar(&config.ShowStats, "s", false, "Show detailed statistics after execution")
	fs.BoolVar(&config.ShowStats, "stats", false, "Show detailed statistics after execution")

	fs.BoolVar(&config.NoStdin, "n", false, "Skip reading from stdin")
	fs.BoolVar(&config.NoStdin, "no-stdin", false, "Skip reading from stdin")

	// Handle help and version flags
	var showHelp, showVersion, installSystem bool
	fs.BoolVar(&showHelp, "h", false, "Show help")
	fs.BoolVar(&showHelp, "help", false, "Show help")
	fs.BoolVar(&showVersion, "V", false, "Show version")
	fs.BoolVar(&showVersion, "version", false, "Show version")
	fs.BoolVar(&installSystem, "install", false, "Install llmcmd system-wide")

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
	if config.ListPresets {
		// Return minimal config with ConfigFile path for preset loading
		return &Config{ConfigFile: config.ConfigFile}, ErrListPresets
	}
	if installSystem {
		return nil, ErrInstall
	}

	// Copy input files from the custom type
	config.InputFiles = []string(inputFiles)

	// If no input files specified, default to stdin
	if len(config.InputFiles) == 0 {
		config.InputFiles = []string{"-"}
	}

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

	// Validate input files exist (skip stdin)
	for _, inputFile := range config.InputFiles {
		// Skip validation for stdin
		if inputFile == "-" {
			continue
		}
		if _, err := os.Stat(inputFile); os.IsNotExist(err) {
			return fmt.Errorf("input file does not exist: %s", inputFile)
		}
	}

	// Validate output file directory exists if specified (skip stdout)
	if config.OutputFile != "" && config.OutputFile != "-" {
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

DESCRIPTION:
    A secure command-line tool that enables Large Language Models to execute
    text processing tasks using the OpenAI ChatCompletion API with built-in tools.

USAGE:
    llmcmd [OPTIONS] [INSTRUCTIONS]

OPTIONS:
    -p, --prompt <text>     LLM prompt/instructions (free text)
    -r, --preset <key>      Use predefined prompt preset (see --list-presets)
    --list-presets          List available prompt presets and exit
    -i, --input <file>      Input file path (can be specified multiple times)
    -o, --output <file>     Output file path  
    -c, --config <file>     Configuration file path (default: ~/.llmcmdrc)
    -v, --verbose           Enable verbose logging
    -s, --stats             Show detailed statistics after execution
    -n, --no-stdin          Skip reading from stdin
    -h, --help              Show this help message
    -V, --version           Show version information

ARGUMENTS:
    INSTRUCTIONS            Command instructions for the LLM

EXAMPLES:
    # Basic text processing from stdin
    echo "hello world" | llmcmd "Convert to uppercase"
    
    # Using preset prompts  
    cat file1.txt file2.txt | llmcmd -r diff_patch "Generate a unified diff"
    llmcmd -r code_review -i source.go "Review this code for issues"
    
    # File processing
    llmcmd -i input.txt -o output.txt "Summarize this document"
    
    # Multiple file comparison
    llmcmd -i file1.txt -i file2.txt "Compare these files and highlight differences"
    
    # List available presets
    llmcmd --list-presets

CONFIGURATION:
    Configuration priority (highest to lowest):
    1. Command line options
    2. Configuration file (~/.llmcmdrc by default)  
    3. Environment variables

    Config file format (.llmcmdrc):
        openai_api_key=your-api-key-here
        model=gpt-4o-mini
        max_tokens=4096
        temperature=0.1
        max_api_calls=50
        timeout_seconds=300
        max_file_size=10485760
        read_buffer_size=4096
        max_retries=3
        retry_delay_ms=1000

    Environment variables:
        OPENAI_API_KEY          API key for OpenAI
        LLMCMD_MODEL           Model to use (default: gpt-4o-mini)
        LLMCMD_MAX_TOKENS      Maximum tokens per response
        LLMCMD_TEMPERATURE     Model temperature (0.0-2.0)
        LLMCMD_MAX_API_CALLS   Maximum API calls per session
        LLMCMD_TIMEOUT         Timeout in seconds

SECURITY:
    - No external command execution (built-in tools only)
    - File access limited to specified input/output files
    - API rate limiting and timeout controls
    - Memory usage limits for safe operation

PRIVACY WARNING:
    ⚠️  All input data is sent to OpenAI's API for processing
    ⚠️  Do NOT process files containing passwords, API keys, or sensitive data
    ⚠️  Your responsibility to ensure data privacy

BUILT-IN TOOLS:
    - read: Read from files or stdin with line/count controls
    - write: Write to files or stdout with newline options
    - spawn: Execute secure built-in text processing commands
    - exit: Clean program termination

For more information: https://github.com/mako10k/llmcmd
`)
}
