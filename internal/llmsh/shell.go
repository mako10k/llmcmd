package llmsh

import (
	"github.com/mako10k/llmcmd/internal/llmsh/parser"
)

// Version information
const (
	Version = "0.1.0"
	Name    = "llmsh"
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
func (s *Shell) Interactive() error {
	// TODO: Implement interactive mode
	return nil
}
