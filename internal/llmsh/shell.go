package llmsh

import (
	"fmt"
	"io"
	"os"
	"strings"
	
	"github.com/chzyer/readline"
	"github.com/mako10k/llmcmd/internal/llmsh/parser"
)

// Version information
var (
	Version     = "3.1.1" // Will be overridden by build-time ldflags
	BuildCommit = "unknown" // Will be overridden by build-time ldflags
	BuildTime   = "unknown" // Will be overridden by build-time ldflags
	Name        = "llmsh"
	Description = "Minimal shell for LLM text processing"
)

// Shell represents the main shell instance
type Shell struct {
	// Configuration
	config *Config

	// Virtual filesystem for pipes and temporary files
	vfs *VirtualFileSystem

	// Command executor
	executor *Executor

	// Parser for shell syntax
	parser *parser.Parser

	// Help system
	help *HelpSystem
}

// Config holds shell configuration
type Config struct {
	// Allowed input/output files from command line
	InputFile  string
	OutputFile string

	// Quota management (inherited from parent llmcmd)
	QuotaManager interface{}

	// Debug mode
	Debug bool
}

// NewShell creates a new shell instance
func NewShell(config *Config) (*Shell, error) {
	if config == nil {
		config = &Config{}
	}

	// Initialize components
	vfs := NewVirtualFileSystem(config.InputFile, config.OutputFile)
	help := NewHelpSystem()
	parser := parser.NewParser()
	executor := NewExecutor(vfs, help, config.QuotaManager)

	return &Shell{
		config:   config,
		vfs:      vfs,
		executor: executor,
		parser:   parser,
		help:     help,
	}, nil
}

// Execute runs a shell command or script
func (s *Shell) Execute(input string) error {
	// Parse the input
	ast, err := s.parser.Parse(input)
	if err != nil {
		return err
	}

	// Execute the parsed commands
	return s.executor.Execute(ast)
}

// Interactive starts an interactive shell session
// This is only called when we know we're in a TTY environment
func (s *Shell) Interactive() error {
	return s.interactiveWithReadline()
}

// interactiveWithReadline handles TTY interactive mode with readline support
func (s *Shell) interactiveWithReadline() error {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "llmsh> ",
		HistoryFile:     os.ExpandEnv("$HOME/.llmsh_history"),
		AutoComplete:    s.createCompleter(),
		InterruptPrompt: "",        // Don't show ^C message
		EOFPrompt:       "",        // Don't show exit message
		VimMode:         false,     // Use emacs mode
	})
	if err != nil {
		return fmt.Errorf("failed to create readline: %v", err)
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				// Ctrl+C pressed, just continue to next prompt
				continue
			} else if err == io.EOF {
				// Ctrl+D or EOF, exit gracefully
				fmt.Println("") // Print newline before exit
				break
			}
			return err
		}

		input := strings.TrimSpace(line)
		
		// Handle special commands
		switch input {
		case "exit", "quit":
			return nil
		case "":
			continue // Empty line, continue
		case "help":
			fmt.Print(s.help.FormatCommandList())
			continue
		}

		// Execute the command
		err = s.Execute(input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}
	
	return nil
}

// createCompleter creates an autocomplete function for readline
func (s *Shell) createCompleter() readline.AutoCompleter {
	// Get available commands from help system - these are the actual implemented commands
	commands := []string{
		"help", "exit", "quit",
		// Basic text processing
		"cat", "echo", "grep", "head", "tail", "sort", "wc", "tr", "cut", "uniq",
		// Data conversion  
		"base64", "od", "hexdump",
		// Basic utilities
		"printf", "true", "false", "test", "yes", "basename", "dirname",
		// Special commands that actually work
		"llmcmd", "llmsh",
	}
	
	items := make([]readline.PrefixCompleterInterface, len(commands))
	for i, cmd := range commands {
		items[i] = readline.PcItem(cmd)
	}
	
	return readline.NewPrefixCompleter(items...)
}
