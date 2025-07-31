package llmsh

import (
	"fmt"
	"io"
	"strings"
	
	"github.com/mako10k/llmcmd/internal/llmsh/parser"
	"github.com/mako10k/llmcmd/internal/tools/builtin"
)

// Executor executes parsed shell commands
type Executor struct {
	vfs          *VirtualFileSystem
	help         *HelpSystem
	quotaManager interface{} // Will be properly typed later
	commands     *Commands
}

// NewExecutor creates a new executor
func NewExecutor(vfs *VirtualFileSystem, help *HelpSystem, quotaManager interface{}) *Executor {
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
		writer, err := e.vfs.OpenForWrite(redir.Target, false)
		if err != nil {
			return err
		}
		redirections["stdout"] = writer.(io.ReadWriteCloser)
		
	case parser.RedirAppend:
		writer, err := e.vfs.OpenForWrite(redir.Target, true)
		if err != nil {
			return err
		}
		redirections["stdout"] = writer.(io.ReadWriteCloser)
		
	case parser.RedirIn:
		reader, err := e.vfs.OpenForRead(redir.Target)
		if err != nil {
			return err
		}
		redirections["stdin"] = reader.(io.ReadWriteCloser)
		
	case parser.RedirErr:
		writer, err := e.vfs.OpenForWrite(redir.Target, false)
		if err != nil {
			return err
		}
		redirections["stderr"] = writer.(io.ReadWriteCloser)
		
	case parser.RedirAll:
		writer, err := e.vfs.OpenForWrite(redir.Target, false)
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
		reader, err := e.vfs.OpenForRead("stdin")
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
		writer, err := e.vfs.OpenForWrite("stdout", false)
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
		writer, err := e.vfs.OpenForWrite("stderr", false)
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
	vfs          *VirtualFileSystem
	help         *HelpSystem
	quotaManager interface{}
}

// NewCommands creates a new command manager
func NewCommands(vfs *VirtualFileSystem, help *HelpSystem, quotaManager interface{}) *Commands {
	return &Commands{
		vfs:          vfs,
		help:         help,
		quotaManager: quotaManager,
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
		// List all commands
		output := c.help.FormatCommandList()
		_, err := stdout.Write([]byte(output))
		return err
	}
	
	// Get help for specific command
	helpText, err := c.help.FormatHelp(args[0])
	if err != nil {
		return err
	}
	
	_, err = stdout.Write([]byte(helpText))
	return err
}

// executeLLMCmd executes llmcmd (recursive LLM execution)
func (c *Commands) executeLLMCmd(args []string, stdin io.ReadWriteCloser, stdout, stderr io.ReadWriteCloser) error {
	// TODO: Implement recursive LLM execution with gpt-4o-mini
	if len(args) == 0 {
		return fmt.Errorf("llmcmd requires a prompt argument")
	}
	
	prompt := strings.Join(args, " ")
	
	// For now, just echo the prompt with a prefix
	output := fmt.Sprintf("[LLM processing] %s\n", prompt)
	_, err := stdout.Write([]byte(output))
	return err
}

// executeLLMSh executes llmsh subshell
func (c *Commands) executeLLMSh(args []string, stdin io.ReadWriteCloser, stdout, stderr io.ReadWriteCloser) error {
	// TODO: Implement llmsh subshell execution
	output := "[llmsh subshell not yet implemented]\n"
	_, err := stdout.Write([]byte(output))
	return err
}

// executeLLMCommand executes LLM-based commands (echo, printf, test, etc.)
func (c *Commands) executeLLMCommand(name string, args []string, stdin io.ReadWriteCloser, stdout, stderr io.ReadWriteCloser) error {
	// TODO: Implement LLM-based command simulation
	
	// Basic implementation for some common commands
	switch name {
	case "echo":
		return c.executeEcho(args, stdout)
	case "printf":
		return c.executePrintf(args, stdout)
	case "true":
		return nil
	case "false":
		return fmt.Errorf("command failed")
	case "test", "[":
		return c.executeTest(args)
	default:
		return fmt.Errorf("command not found: %s", name)
	}
}

// executeEcho implements basic echo command
func (c *Commands) executeEcho(args []string, stdout io.ReadWriteCloser) error {
	var output string
	var noNewline bool
	
	// Parse -n flag
	startIdx := 0
	if len(args) > 0 && args[0] == "-n" {
		noNewline = true
		startIdx = 1
	}
	
	if len(args) > startIdx {
		output = strings.Join(args[startIdx:], " ")
	}
	
	if !noNewline {
		output += "\n"
	}
	
	_, err := stdout.Write([]byte(output))
	return err
}

// executePrintf implements basic printf command
func (c *Commands) executePrintf(args []string, stdout io.ReadWriteCloser) error {
	if len(args) == 0 {
		return fmt.Errorf("printf: missing format string")
	}
	
	// Very basic printf implementation
	format := args[0]
	format = strings.ReplaceAll(format, "\\n", "\n")
	format = strings.ReplaceAll(format, "\\t", "\t")
	
	_, err := stdout.Write([]byte(format))
	return err
}

// executeTest implements basic test command
func (c *Commands) executeTest(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("test failed: no arguments")
	}
	
	// Very basic test implementation
	if len(args) == 3 && args[1] == "=" {
		if args[0] == args[2] {
			return nil
		}
		return fmt.Errorf("test failed: strings not equal")
	}
	
	if len(args) == 2 && args[0] == "-z" {
		if args[1] == "" {
			return nil
		}
		return fmt.Errorf("test failed: string not empty")
	}
	
	return fmt.Errorf("test: unsupported expression")
}
