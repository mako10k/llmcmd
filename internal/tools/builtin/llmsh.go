package builtin

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// Llmsh executes the llmsh command in a separate process
func Llmsh(args []string, stdin io.Reader, stdout io.Writer) error {
	if handled, _, err := HandleHelp(args, stdout, `llmsh - Execute llmsh shell in subprocess

Usage: llmsh [llmsh-args...]

Description:
	Execute llmsh shell command in a separate forked process

Options:
	--help, -h        Show this help message

Examples:
	llmsh                     Start interactive shell
	llmsh -c "command"        Execute command in shell
`); handled {
		return err
	}

	// Parse arguments for llmsh execution
	processedArgs, err := parseLlmshArgs(args)
	if err != nil {
		return fmt.Errorf("llmsh: argument parsing failed: %v", err)
	}

	// Fork child process for llmsh execution
	return executeLlmshInFork(processedArgs, stdin, stdout)
}

// executeLlmshInFork executes llmsh in a forked child process
func executeLlmshInFork(args []string, stdin io.Reader, stdout io.Writer) error {
	// Validate llmsh can be executed first
	if err := validateLlmshExecution(); err != nil {
		return err
	}

	// Create command to execute llmsh
	cmd := exec.Command("llmsh", args...)

	// Setup stdio pipes
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stdout

	// Set process group for proper isolation and signal handling
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:   true,            // Create new process group
		Pdeathsig: syscall.SIGTERM, // Send SIGTERM when parent dies
	}

	// Set environment for llmsh execution with isolation
	cmd.Env = setupLlmshEnvironment()

	// Execute command and wait for completion
	return executeSubProcess(cmd, "llmsh")
}

// parseLlmshArgs parses arguments for llmsh command
func parseLlmshArgs(args []string) ([]string, error) {
	var processedArgs []string

	for i, arg := range args {
		// Handle special cases for argument processing
		if strings.HasPrefix(arg, "-") {
			// Validate known flags for llmsh
			if !isValidLlmshFlag(arg) {
				return nil, fmt.Errorf("invalid llmsh flag: %s", arg)
			}
			processedArgs = append(processedArgs, arg)
		} else {
			// For non-flag arguments (scripts, commands), validate and pass through
			if arg == "" {
				return nil, fmt.Errorf("empty argument at position %d", i)
			}
			processedArgs = append(processedArgs, arg)
		}
	}

	return processedArgs, nil
}

// isValidLlmshFlag validates if a flag is valid for llmsh
func isValidLlmshFlag(flag string) bool {
	validFlags := map[string]bool{
		"-c":            true, // Execute command
		"--command":     true, // Execute command (long form)
		"-i":            true, // Interactive mode
		"--interactive": true, // Interactive mode (long form)
		"-v":            true, // Verbose mode
		"--verbose":     true, // Verbose mode (long form)
		"-h":            true, // Help
		"--help":        true, // Help (long form)
		"--version":     true, // Version information
	}

	// Handle flags with values (e.g., -c "command")
	if strings.Contains(flag, "=") {
		parts := strings.SplitN(flag, "=", 2)
		return validFlags[parts[0]]
	}

	return validFlags[flag]
}

// validateLlmshExecution validates that llmsh can be executed
func validateLlmshExecution() error {
	// Check if llmsh binary exists in PATH
	_, err := exec.LookPath("llmsh")
	if err != nil {
		return fmt.Errorf("llmsh binary not found in PATH: %v", err)
	}

	return nil
}

// setupLlmshEnvironment sets up environment for llmsh execution
func setupLlmshEnvironment() []string {
	// Inherit current environment but ensure clean state
	env := os.Environ()

	// Add llmsh-specific environment variables for fork mode
	env = append(env, "LLMSH_FORK_MODE=1")
	env = append(env, "LLMSH_PROCESS_ISOLATION=1")

	return env
}
