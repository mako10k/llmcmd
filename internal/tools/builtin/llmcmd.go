package builtin

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// Llmcmd executes the llmcmd command in a separate process
func Llmcmd(args []string, stdin io.Reader, stdout io.Writer) error {
		if handled, _, err := HandleHelp(args, stdout, `llmcmd - Execute llmcmd in subprocess

Usage: llmcmd [llmcmd-args...]

Description:
	Execute llmcmd command in a separate forked process

Options:
	--help, -h        Show this help message

Examples:
	llmcmd read file.txt      Execute llmcmd read command
	llmcmd help               Show llmcmd help
`); handled {
				return err
		}

	// Parse arguments for llmcmd execution
	if len(args) == 0 {
		return fmt.Errorf("llmcmd: no arguments provided")
	}

	// Fork child process for llmcmd execution
	return executeLlmcmdInFork(args, stdin, stdout)
}

// executeLlmcmdInFork executes llmcmd in a forked child process
func executeLlmcmdInFork(args []string, stdin io.Reader, stdout io.Writer) error {
	// Validate llmcmd can be executed first
	if err := validateLlmcmdExecution(); err != nil {
		return err
	}

	// Process arguments for security and validation
	processedArgs, err := parseLlmcmdArgs(args)
	if err != nil {
		return err
	}

	// Create command to execute llmcmd binary
	cmd := exec.Command("llmcmd", processedArgs...)

	// Setup stdio pipes
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stdout

	// Set process group for proper isolation and signal handling
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:   true,            // Create new process group
		Pdeathsig: syscall.SIGTERM, // Send SIGTERM when parent dies
	}

	// Set environment for isolated execution
	cmd.Env = setupLlmcmdEnvironment()

	// Execute command and wait for completion
	return executeSubProcess(cmd, "llmcmd")
}

// parseLlmcmdArgs parses arguments for llmcmd command
func parseLlmcmdArgs(args []string) ([]string, error) {
	var processedArgs []string

	for i, arg := range args {
		// Handle special cases for argument processing
		if strings.HasPrefix(arg, "-") {
			// Pass through flags as-is
			processedArgs = append(processedArgs, arg)
		} else {
			// For non-flag arguments, validate and pass through
			if arg == "" {
				return nil, fmt.Errorf("empty argument at position %d", i)
			}
			processedArgs = append(processedArgs, arg)
		}
	}

	return processedArgs, nil
}

// validateLlmcmdExecution validates that llmcmd can be executed
func validateLlmcmdExecution() error {
	// Check if llmcmd binary exists in PATH
	_, err := exec.LookPath("llmcmd")
	if err != nil {
		return fmt.Errorf("llmcmd binary not found in PATH: %v", err)
	}

	return nil
}

// setupLlmcmdEnvironment sets up environment for llmcmd execution
func setupLlmcmdEnvironment() []string {
	// Inherit current environment but ensure clean state
	env := os.Environ()

	// Add any llmcmd-specific environment variables if needed
	env = append(env, "LLMCMD_FORK_MODE=1")
	env = append(env, "LLMCMD_PROCESS_ISOLATION=1")

	return env
}
