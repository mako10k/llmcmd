package llmsh

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mako10k/llmcmd/internal/app"
	"github.com/mako10k/llmcmd/internal/llmsh/commands"
	"github.com/mako10k/llmcmd/internal/llmsh/parser"
	"github.com/mako10k/llmcmd/internal/openai"
	"github.com/mako10k/llmcmd/internal/tools/builtin"
)

// Executor executes parsed shell commands
type Executor struct {
	vfs          *app.VirtualFS
	help         *HelpSystem
	quotaManager interface{} // Will be properly typed later
	commands     *Commands
}

// NewExecutor creates a new executor
func NewExecutor(vfs *app.VirtualFS, help *HelpSystem, quotaManager interface{}) *Executor {
	// Configure builtin commands with VFS
	builtin.SetVFS(vfs)

	return &Executor{
		vfs:          vfs,
		help:         help,
		quotaManager: quotaManager,
		commands:     NewCommands(vfs, help, quotaManager),
	}
}

// Execute executes a parsed AST node
func (e *Executor) Execute(node parser.Node) error {
	if node == nil {
		return nil
	}

	switch n := node.(type) {
	case *parser.ScriptNode:
		return e.executeScript(n)
	case *parser.SequenceNode:
		return e.executeSequence(n)
	case *parser.ConditionalNode:
		return e.executeConditional(n)
	case *parser.ComplexCommandNode:
		return e.executeComplexCommand(n)
	case *parser.PipelineNode:
		return e.executePipeline(n, nil)
	case *parser.CommandNode:
		return e.executeCommand(n, nil, nil, nil)
	default:
		return fmt.Errorf("unknown node type: %T", node)
	}
}

// executeScript executes a script (multiple statements)
func (e *Executor) executeScript(script *parser.ScriptNode) error {
	for _, stmt := range script.Statements {
		if err := e.Execute(stmt); err != nil {
			return err
		}
	}
	return nil
}

// executeSequence executes sequential commands
func (e *Executor) executeSequence(seq *parser.SequenceNode) error {
	for _, cmd := range seq.Commands {
		if err := e.Execute(cmd); err != nil {
			return err
		}
	}
	return nil
}

// executeConditional executes conditional commands (&& or ||)
func (e *Executor) executeConditional(cond *parser.ConditionalNode) error {
	leftErr := e.Execute(cond.Left)

	switch cond.Operator {
	case "&&":
		// Execute right only if left succeeded
		if leftErr == nil {
			return e.Execute(cond.Right)
		}
		return leftErr

	case "||":
		// Execute right only if left failed
		if leftErr != nil {
			return e.Execute(cond.Right)
		}
		return nil

	default:
		return fmt.Errorf("unknown conditional operator: %s", cond.Operator)
	}
}

// executeComplexCommand executes a command with redirections
func (e *Executor) executeComplexCommand(complex *parser.ComplexCommandNode) error {
	redirections := make(map[string]io.ReadWriteCloser)

	// Set up redirections
	for _, redir := range complex.Redirections {
		if err := e.setupRedirection(redir, redirections); err != nil {
			e.cleanupRedirections(redirections)
			return err
		}
	}

	// Execute the pipeline with redirections
	err := e.executePipeline(complex.Pipeline, redirections)

	// Clean up redirections
	e.cleanupRedirections(redirections)

	return err
}

// setupRedirection sets up a single redirection
func (e *Executor) setupRedirection(redir *parser.RedirectionNode, redirections map[string]io.ReadWriteCloser) error {
	switch redir.Type {
	case parser.RedirOut:
		writer, err := e.vfs.OpenForWrite(redir.Target, false, true)
		if err != nil {
			return err
		}
		redirections["stdout"] = writer.(io.ReadWriteCloser)

	case parser.RedirAppend:
		writer, err := e.vfs.OpenForWrite(redir.Target, true, true)
		if err != nil {
			return err
		}
		redirections["stdout"] = writer.(io.ReadWriteCloser)

	case parser.RedirIn:
		reader, err := e.vfs.OpenForRead(redir.Target, true)
		if err != nil {
			return err
		}
		redirections["stdin"] = reader.(io.ReadWriteCloser)

	case parser.RedirErr:
		writer, err := e.vfs.OpenForWrite(redir.Target, false, true)
		if err != nil {
			return err
		}
		redirections["stderr"] = writer.(io.ReadWriteCloser)

	case parser.RedirAll:
		writer, err := e.vfs.OpenForWrite(redir.Target, false, true)
		if err != nil {
			return err
		}
		redirections["stdout"] = writer.(io.ReadWriteCloser)
		redirections["stderr"] = writer.(io.ReadWriteCloser)

	default:
		return fmt.Errorf("unknown redirection type")
	}

	return nil
}

// cleanupRedirections closes all redirection files
func (e *Executor) cleanupRedirections(redirections map[string]io.ReadWriteCloser) {
	for _, file := range redirections {
		if file != nil {
			file.Close()
		}
	}
}

// executePipeline executes a pipeline of commands
func (e *Executor) executePipeline(pipeline *parser.PipelineNode, redirections map[string]io.ReadWriteCloser) error {
	if len(pipeline.Commands) == 0 {
		return nil
	}

	if len(pipeline.Commands) == 1 {
		// Single command
		var stdin, stdout, stderr io.ReadWriteCloser

		// Use redirections or defaults
		if redirections != nil {
			stdin = redirections["stdin"]
			stdout = redirections["stdout"]
			stderr = redirections["stderr"]
		}

		return e.executeCommand(pipeline.Commands[0], stdin, stdout, stderr)
	}

	// Multiple commands - create pipes
	var readers []io.ReadCloser
	var writers []io.WriteCloser

	for i := 0; i < len(pipeline.Commands)-1; i++ {
		reader, writer, err := e.vfs.CreatePipe()
		if err != nil {
			// Clean up already created pipes
			for _, r := range readers {
				r.Close()
			}
			for _, w := range writers {
				w.Close()
			}
			return err
		}
		readers = append(readers, reader)
		writers = append(writers, writer)
	}

	// Execute commands in pipeline
	for i, cmd := range pipeline.Commands {
		var stdin, stdout, stderr io.ReadWriteCloser

		// Set up stdin
		if i == 0 {
			// First command uses input redirection or default stdin
			if redirections != nil {
				stdin = redirections["stdin"]
			}
		} else {
			// Use pipe from previous command
			if rwc, ok := readers[i-1].(io.ReadWriteCloser); ok {
				stdin = rwc
			} else {
				return fmt.Errorf("pipe reader does not support read/write")
			}
		}

		// Set up stdout
		if i == len(pipeline.Commands)-1 {
			// Last command uses output redirection or default stdout
			if redirections != nil {
				stdout = redirections["stdout"]
			}
		} else {
			// Use pipe to next command
			if rwc, ok := writers[i].(io.ReadWriteCloser); ok {
				stdout = rwc
			} else {
				return fmt.Errorf("pipe writer does not support read/write")
			}
		}

		// stderr always uses redirection or default
		if redirections != nil {
			stderr = redirections["stderr"]
		}

		if err := e.executeCommand(cmd, stdin, stdout, stderr); err != nil {
			// Clean up pipes
			for _, r := range readers {
				r.Close()
			}
			for _, w := range writers {
				w.Close()
			}
			return err
		}
	}

	// Clean up pipes
	for _, r := range readers {
		r.Close()
	}
	for _, w := range writers {
		w.Close()
	}

	return nil
}

// executeCommand executes a single command
func (e *Executor) executeCommand(cmd *parser.CommandNode, stdin, stdout, stderr io.ReadWriteCloser) error {
	// Use default streams if not provided
	if stdin == nil {
		reader, err := e.vfs.OpenForRead("stdin", true)
		if err != nil {
			return err
		}
		// For now, we'll use a type assertion - this needs better design
		if rwc, ok := reader.(io.ReadWriteCloser); ok {
			stdin = rwc
		} else {
			return fmt.Errorf("stdin does not support read/write")
		}
	}
	if stdout == nil {
		writer, err := e.vfs.OpenForWrite("stdout", false, true)
		if err != nil {
			return err
		}
		// For now, we'll use a type assertion - this needs better design
		if rwc, ok := writer.(io.ReadWriteCloser); ok {
			stdout = rwc
		} else {
			return fmt.Errorf("stdout does not support read/write")
		}
	}
	if stderr == nil {
		writer, err := e.vfs.OpenForWrite("stderr", false, true)
		if err != nil {
			return err
		}
		// For now, we'll use a type assertion - this needs better design
		if rwc, ok := writer.(io.ReadWriteCloser); ok {
			stderr = rwc
		} else {
			return fmt.Errorf("stderr does not support read/write")
		}
	}

	return e.commands.Execute(cmd.Name, cmd.Args, stdin, stdout, stderr)
}

// Commands manages command execution
type Commands struct {
	vfs          *app.VirtualFS
	help         *HelpSystem
	quotaManager interface{}
	manager      *commands.Manager
	sharedQuota  *openai.SharedQuotaManager // For llmcmd quota sharing
}

// NewCommands creates a new command manager
func NewCommands(vfs *app.VirtualFS, help *HelpSystem, quotaManager interface{}) *Commands {
	// Create shared quota manager for llmcmd calls
	// TODO: This should use actual quota configuration
	defaultQuotaConfig := &openai.QuotaConfig{
		MaxTokens:    4096,
		InputWeight:  1.0,
		CachedWeight: 0.25,
		OutputWeight: 4.0,
	}
	sharedQuota := openai.NewSharedQuotaManager(defaultQuotaConfig)

	return &Commands{
		vfs:          vfs,
		help:         help,
		quotaManager: quotaManager,
		manager:      commands.NewManager(),
		sharedQuota:  sharedQuota,
	}
}

// Execute executes a command by name
func (c *Commands) Execute(name string, args []string, stdin io.ReadWriteCloser, stdout, stderr io.ReadWriteCloser) error {
	// Handle special commands first
	switch name {
	case "help", "man":
		return c.executeHelp(args, stdout)
	case "llmcmd":
		return c.executeLLMCmd(args, stdin, stdout, stderr)
	case "llmsh":
		return c.executeLLMSh(args, stdin, stdout, stderr)
	}

	// Check new internal command implementations first
	if c.manager.IsInternalCommand(name) {
		return c.manager.Execute(name, args, stdin, stdout)
	}

	// Check built-in commands
	if builtinFunc, exists := builtin.Commands[name]; exists {
		return builtinFunc(args, stdin, stdout)
	}

	// Check LLM-based commands
	return c.executeLLMCommand(name, args, stdin, stdout, stderr)
}

// executeHelp executes help command
func (c *Commands) executeHelp(args []string, stdout io.ReadWriteCloser) error {
	if len(args) == 0 {
		// List all commands; defensively init help system if nil
		if c.help == nil {
			c.help = NewHelpSystem()
		}
		output := c.help.FormatCommandList()
		_, err := stdout.Write([]byte(output))
		return err
	}

	// Get help for specific command
	if c.help == nil {
		c.help = NewHelpSystem()
	}
	helpText, err := c.help.FormatHelp(args[0])
	if err != nil {
		return err
	}

	_, err = stdout.Write([]byte(helpText))
	return err
}

// executeLLMCmd executes llmcmd (recursive LLM execution)
func (c *Commands) executeLLMCmd(args []string, stdin io.ReadWriteCloser, stdout, stderr io.ReadWriteCloser) error {
	if len(args) == 0 {
		return fmt.Errorf("llmcmd requires a prompt argument")
	}

	// Read input from stdin if available
	input, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("llmcmd: error reading input: %w", err)
	}

	// Prepare llmcmd arguments
	var llmcmdArgs []string
	if len(input) > 0 {
		// If we have stdin input, pass it via -i flag
		llmcmdArgs = append(llmcmdArgs, "-i", string(input), "-p", strings.Join(args, " "))
	} else {
		llmcmdArgs = append(llmcmdArgs, "-p", strings.Join(args, " "))
	}

	// Generate process ID for this llmcmd call
	processID := fmt.Sprintf("llmcmd-%d", time.Now().UnixNano())
	parentID := "llmsh"

	// Register process with shared quota
	if err := c.sharedQuota.RegisterProcess(processID, parentID); err != nil {
		return fmt.Errorf("llmcmd: failed to register process: %w", err)
	}
	defer c.sharedQuota.UnregisterProcess(processID)

	// Check quota before execution
	if !c.sharedQuota.CanMakeCall(processID) {
		return fmt.Errorf("llmcmd: quota exceeded")
	}

	// Execute llmcmd internally with shared quota
	metadata := app.ApplicationMetadata{
		Name:    "llmcmd",
		Version: "3.0.3",
	}

	// Execute with internal context
	// Nested llmcmd calls from llmsh are not top-level
	isTopLevel := false // llmsh is always a nested execution context
	err = app.ExecuteInternal(metadata, llmcmdArgs, c.sharedQuota, processID, parentID, isTopLevel)
	if err != nil {
		return fmt.Errorf("llmcmd: execution failed: %w", err)
	}

	return nil
}

// executeLLMSh executes llmsh subshell
func (c *Commands) executeLLMSh(args []string, stdin io.ReadWriteCloser, stdout, stderr io.ReadWriteCloser) error {
	// Generate process ID for this llmsh call
	processID := fmt.Sprintf("llmsh-%d", time.Now().UnixNano())
	parentID := "llmsh-parent"

	// Register process with shared quota
	if err := c.sharedQuota.RegisterProcess(processID, parentID); err != nil {
		return fmt.Errorf("llmsh: failed to register process: %w", err)
	}
	defer c.sharedQuota.UnregisterProcess(processID)

	// Handle -c option for command execution
	if len(args) >= 2 && args[0] == "-c" {
		// Join all arguments after -c as the command
		command := strings.Join(args[1:], " ")

		// Create a new shell instance for the subshell with shared quota
		config := &Config{}
		subShell, err := NewShell(config)
		if err != nil {
			return fmt.Errorf("failed to create subshell: %w", err)
		}

		// Set shared quota in subshell (if Shell supports it)
		// For now, execute the command in the subshell
		return subShell.Execute(command)
	}

	// If no -c option, show help or start interactive mode (not implemented)
	output := "llmsh: interactive mode not yet implemented. Use 'llmsh -c \"command\"' to execute commands.\n"
	_, err := stderr.Write([]byte(output))
	return err
}

// executeLLMCommand executes remaining LLM-based commands not migrated to new packages
func (c *Commands) executeLLMCommand(name string, args []string, stdin io.ReadWriteCloser, stdout, stderr io.ReadWriteCloser) error {
	// Only commands not implemented in the new command packages
	switch name {
	default:
		return fmt.Errorf("command not found: %s", name)
	}
}
