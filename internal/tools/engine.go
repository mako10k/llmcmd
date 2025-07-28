package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mako10k/llmcmd/internal/tools/builtin"
)

// PipeArgs represents arguments for pipe command
type PipeArgs struct {
	Commands []PipeCommand `json:"commands"`
	Input    PipeInput     `json:"input"`
}

// PipeCommand represents a single command in a pipeline
type PipeCommand struct {
	Name string   `json:"name"`
	Args []string `json:"args"`
}

// PipeInput represents input source for pipeline
type PipeInput struct {
	Type string `json:"type"` // "fd" or "data"
	FD   int    `json:"fd"`   // file descriptor number
	Data string `json:"data"` // raw input data
}

// Engine handles tool execution for llmcmd
type Engine struct {
	inputFiles      []*os.File
	outputFile      *os.File
	fileDescriptors []io.Reader
	maxFileSize     int64
	bufferSize      int
	stats           ExecutionStats
	noStdin         bool // Skip reading from stdin
	noNewline       bool // Don't add newline to output
}

// ExecutionStats tracks tool execution statistics
type ExecutionStats struct {
	ReadCalls    int   `json:"read_calls"`
	WriteCalls   int   `json:"write_calls"`
	PipeCalls    int   `json:"pipe_calls"`
	ExitCalls    int   `json:"exit_calls"`
	BytesRead    int64 `json:"bytes_read"`
	BytesWritten int64 `json:"bytes_written"`
	ErrorCount   int   `json:"error_count"`
}

// EngineConfig holds configuration for the tool engine
type EngineConfig struct {
	InputFiles  []string
	OutputFile  string
	MaxFileSize int64
	BufferSize  int
	NoStdin     bool // Skip reading from stdin
	NoNewline   bool // Don't add newline to output
}

// NewEngine creates a new tool execution engine
func NewEngine(config EngineConfig) (*Engine, error) {
	engine := &Engine{
		maxFileSize: config.MaxFileSize,
		bufferSize:  config.BufferSize,
		noStdin:     config.NoStdin,
		noNewline:   config.NoNewline,
	}

	// Initialize file descriptors array
	// 0=stdin, 1=stdout, 2=stderr, 3+=input files
	engine.fileDescriptors = make([]io.Reader, 3)
	if !config.NoStdin {
		engine.fileDescriptors[0] = os.Stdin
	}

	// Open input files and add to file descriptors
	for _, filename := range config.InputFiles {
		if filename == "-" {
			// "-" means stdin, so add stdin as an additional file descriptor
			engine.fileDescriptors = append(engine.fileDescriptors, os.Stdin)
		} else {
			file, err := os.Open(filename)
			if err != nil {
				return nil, fmt.Errorf("failed to open input file %s: %w", filename, err)
			}
			engine.inputFiles = append(engine.inputFiles, file)
			engine.fileDescriptors = append(engine.fileDescriptors, file)
		}
	}

	// Open output file if specified
	if config.OutputFile != "" {
		if config.OutputFile == "-" {
			// Use stdout for "-"
			engine.outputFile = os.Stdout
		} else {
			file, err := os.Create(config.OutputFile)
			if err != nil {
				return nil, fmt.Errorf("failed to create output file %s: %w", config.OutputFile, err)
			}
			engine.outputFile = file
		}
	}

	return engine, nil
}

// Close closes all file handles
func (e *Engine) Close() error {
	var errors []error

	// Close input files
	for _, file := range e.inputFiles {
		if err := file.Close(); err != nil {
			errors = append(errors, err)
		}
	}

	// Close output file
	if e.outputFile != nil {
		if err := e.outputFile.Close(); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors closing files: %v", errors)
	}
	return nil
}

// ExecuteToolCall executes a tool call and returns the result
func (e *Engine) ExecuteToolCall(toolCall map[string]interface{}) (string, error) {
	// Extract function name
	functionName, ok := toolCall["name"].(string)
	if !ok {
		e.stats.ErrorCount++
		return "", fmt.Errorf("invalid tool call: missing function name")
	}

	// Extract arguments
	argsStr, ok := toolCall["arguments"].(string)
	if !ok {
		e.stats.ErrorCount++
		return "", fmt.Errorf("invalid tool call: missing arguments")
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argsStr), &args); err != nil {
		e.stats.ErrorCount++
		return "", fmt.Errorf("invalid tool call arguments: %w", err)
	}

	// Execute the appropriate function
	switch functionName {
	case "read":
		return e.executeRead(args)
	case "write":
		return e.executeWrite(args)
	case "pipe":
		return e.executePipe(args)
	case "exit":
		return e.executeExit(args)
	default:
		e.stats.ErrorCount++
		return "", fmt.Errorf("unknown function: %s", functionName)
	}
}

// executeRead implements the read tool
func (e *Engine) executeRead(args map[string]interface{}) (string, error) {
	e.stats.ReadCalls++

	// Extract file descriptor
	fdFloat, ok := args["fd"].(float64)
	if !ok {
		e.stats.ErrorCount++
		return "", fmt.Errorf("read: fd parameter must be a number")
	}
	fd := int(fdFloat)

	// Extract count (optional, default to buffer size)
	count := e.bufferSize
	if countFloat, ok := args["count"].(float64); ok {
		count = int(countFloat)
		if count <= 0 || count > e.bufferSize {
			e.stats.ErrorCount++
			return "", fmt.Errorf("read: count must be between 1 and %d", e.bufferSize)
		}
	}

	// Get the appropriate reader
	var reader io.Reader
	if fd < 0 || fd >= len(e.fileDescriptors) {
		e.stats.ErrorCount++
		return "", fmt.Errorf("read: invalid file descriptor %d", fd)
	}
	
	reader = e.fileDescriptors[fd]
	if reader == nil {
		e.stats.ErrorCount++
		return "", fmt.Errorf("read: file descriptor %d not available", fd)
	}

	// Read data with blocking I/O
	buffer := make([]byte, count)
	n, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		e.stats.ErrorCount++
		return "", fmt.Errorf("read: %w", err)
	}

	e.stats.BytesRead += int64(n)
	result := string(buffer[:n])
	
	// Return empty string if no data, but don't treat it as error
	return result, nil
}

// executeWrite implements the write tool
func (e *Engine) executeWrite(args map[string]interface{}) (string, error) {
	e.stats.WriteCalls++

	// Extract file descriptor
	fdFloat, ok := args["fd"].(float64)
	if !ok {
		e.stats.ErrorCount++
		return "", fmt.Errorf("write: fd parameter must be a number")
	}
	fd := int(fdFloat)

	// Extract data
	data, ok := args["data"].(string)
	if !ok {
		e.stats.ErrorCount++
		return "", fmt.Errorf("write: data parameter must be a string")
	}

	// Get the appropriate writer
	var writer io.Writer
	switch fd {
	case 1: // stdout
		if e.outputFile != nil {
			writer = e.outputFile
		} else {
			writer = os.Stdout
		}
	case 2: // stderr
		writer = os.Stderr
	default:
		e.stats.ErrorCount++
		return "", fmt.Errorf("write: invalid file descriptor %d (only 1=stdout, 2=stderr allowed)", fd)
	}

	// Write data
	n, err := writer.Write([]byte(data))
	if err != nil {
		e.stats.ErrorCount++
		return "", fmt.Errorf("write: %w", err)
	}

	e.stats.BytesWritten += int64(n)
	return fmt.Sprintf("wrote %d bytes to fd %d", n, fd), nil
}

// executePipe implements the pipe tool for executing built-in commands
func (e *Engine) executePipe(args map[string]interface{}) (string, error) {
	e.stats.PipeCalls++

	// Parse JSON arguments
	var pipeArgs PipeArgs
	jsonBytes, err := json.Marshal(args)
	if err != nil {
		e.stats.ErrorCount++
		return "", fmt.Errorf("pipe: failed to marshal arguments: %w", err)
	}

	if err := json.Unmarshal(jsonBytes, &pipeArgs); err != nil {
		e.stats.ErrorCount++
		return "", fmt.Errorf("pipe: failed to parse arguments: %w", err)
	}

	if len(pipeArgs.Commands) == 0 {
		e.stats.ErrorCount++
		return "", fmt.Errorf("pipe: no commands specified")
	}

	// Parse input source
	var input io.Reader
	if pipeArgs.Input.Type == "fd" {
		fd := pipeArgs.Input.FD
		if fd < 0 || fd >= len(e.fileDescriptors) {
			e.stats.ErrorCount++
			return "", fmt.Errorf("pipe: invalid file descriptor: %d", fd)
		}
		if e.fileDescriptors[fd] == nil {
			e.stats.ErrorCount++
			return "", fmt.Errorf("pipe: file descriptor %d not open", fd)
		}
		input = e.fileDescriptors[fd]
	} else if pipeArgs.Input.Type == "data" {
		input = strings.NewReader(pipeArgs.Input.Data)
	} else {
		e.stats.ErrorCount++
		return "", fmt.Errorf("pipe: invalid input type: %s", pipeArgs.Input.Type)
	}

	// Execute pipeline
	currentInput := input
	for i, cmd := range pipeArgs.Commands {
		cmdName := cmd.Name
		cmdArgs := cmd.Args

		// Check if command is supported
		commandFunc, exists := builtin.Commands[cmdName]
		if !exists {
			e.stats.ErrorCount++
			return "", fmt.Errorf("pipe: unsupported command: %s", cmdName)
		}

		// Prepare output buffer
		var output bytes.Buffer
		
		// Execute command
		if err := commandFunc(cmdArgs, currentInput, &output); err != nil {
			e.stats.ErrorCount++
			return "", fmt.Errorf("pipe: command '%s' failed: %w", cmdName, err)
		}

		// Use output as input for next command
		if i < len(pipeArgs.Commands)-1 {
			currentInput = strings.NewReader(output.String())
		} else {
			// Last command - return the result
			return strings.TrimSuffix(output.String(), "\n"), nil
		}
	}

	return "", nil
}

// executeExit implements the exit tool
func (e *Engine) executeExit(args map[string]interface{}) (string, error) {
	e.stats.ExitCalls++

	// Extract exit code
	codeFloat, ok := args["code"].(float64)
	if !ok {
		e.stats.ErrorCount++
		return "", fmt.Errorf("exit: code parameter must be a number")
	}
	code := int(codeFloat)

	// Extract message (optional)
	message := ""
	if msg, ok := args["message"].(string); ok {
		message = msg
	}

	if message != "" {
		fmt.Fprintf(os.Stderr, "%s\n", message)
	}

	// Return a special error to indicate exit request instead of calling os.Exit directly
	return fmt.Sprintf("Exit requested with code %d", code), fmt.Errorf("EXIT_REQUESTED:%d", code)
}

// GetStats returns current execution statistics
func (e *Engine) GetStats() ExecutionStats {
	return e.stats
}
