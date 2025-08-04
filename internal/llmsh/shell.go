package llmsh

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chzyer/readline"
	"github.com/mako10k/llmcmd/internal/app"
	"github.com/mako10k/llmcmd/internal/llmsh/parser"
)

// Constants for the shell
// Version information
var (
	Version     = "3.1.1"   // Will be overridden by build-time ldflags
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
	InputFiles  []string
	OutputFiles []string

	// Virtual mode flag (determines isTopLevelCmd behavior)
	VirtualMode bool

	// Quota management (inherited from parent llmcmd)
	QuotaManager interface{}

	// Debug mode
	Debug bool

	// FSProxy integration settings (Phase 3.1)
	EnableFSProxy  bool
	FSProxyManager interface{} // Should be *app.FSProxyManager, but avoiding circular import
	FSProxyVFSMode bool        // Whether to restrict file access to VFS only
}

// NewShell creates a new shell instance
func NewShell(config *Config) (*Shell, error) {
	if config == nil {
		config = &Config{}
	}

	// Initialize VFS with optional FSProxy integration
	var vfs *VirtualFileSystem
	if config.EnableFSProxy && config.FSProxyManager != nil {
		// Create FSProxy adapter
		fsProxyManager, ok := config.FSProxyManager.(*app.FSProxyManager)
		if !ok {
			return nil, fmt.Errorf("invalid FSProxyManager type")
		}

		// Create legacy VFS for fallback
		legacyVFS := NewVirtualFileSystem(config.InputFiles, config.OutputFiles)

		// Create adapter with FSProxy support
		adapter := app.NewVFSFSProxyAdapter(fsProxyManager, legacyVFS, true)

		// Create enhanced VFS with FSProxy integration
		vfs = NewVirtualFileSystemWithFSProxy(config.InputFiles, config.OutputFiles, true, adapter)
	} else {
		// Use legacy VFS
		vfs = NewVirtualFileSystem(config.InputFiles, config.OutputFiles)
	}

	// Initialize other components
	parserInstance := parser.NewParser()
	executor := NewExecutor(vfs, nil, config.QuotaManager)

	return &Shell{
		config:   config,
		vfs:      vfs,
		executor: executor,
		parser:   parserInstance,
		help:     nil,
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

// Interactive starts the interactive shell mode
func (s *Shell) Interactive() error {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:            "llmsh> ",
		HistoryFile:       "",
		HistoryLimit:      1000,
		HistorySearchFold: true,
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
	})
	if err != nil {
		return err
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if line == "exit" || line == "quit" {
			break
		}

		if line == "help" {
			fmt.Println("Available commands: exit, quit, help")
			fmt.Println("Or enter any shell command or pipeline")
			continue
		}

		// Execute the command
		err = s.Execute(line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}

	return nil
}

// Config represents shell configuration

// interactiveWithReadline handles TTY interactive mode with readline support
func (s *Shell) interactiveWithReadline() error {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "llmsh> ",
		HistoryFile:     os.ExpandEnv("$HOME/.llmsh_history"),
		AutoComplete:    s.createCompleter(),
		InterruptPrompt: "",    // Don't show ^C message
		EOFPrompt:       "",    // Don't show exit message
		VimMode:         false, // Use emacs mode
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
