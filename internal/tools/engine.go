package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Engine handles tool execution for llmcmd
type Engine struct {
	inputFiles   []*os.File
	outputFile   *os.File
	maxFileSize  int64
	bufferSize   int
	stats        ExecutionStats
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
}

// NewEngine creates a new tool execution engine
func NewEngine(config EngineConfig) (*Engine, error) {
	engine := &Engine{
		maxFileSize: config.MaxFileSize,
		bufferSize:  config.BufferSize,
	}

	// Open input files
	for _, filename := range config.InputFiles {
		file, err := os.Open(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to open input file %s: %w", filename, err)
		}
		engine.inputFiles = append(engine.inputFiles, file)
	}

	// Open output file if specified
	if config.OutputFile != "" {
		file, err := os.Create(config.OutputFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create output file %s: %w", config.OutputFile, err)
		}
		engine.outputFile = file
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
	switch fd {
	case 0: // stdin
		reader = os.Stdin
	default: // input files (fd 3+)
		fileIndex := fd - 3
		if fileIndex < 0 || fileIndex >= len(e.inputFiles) {
			e.stats.ErrorCount++
			return "", fmt.Errorf("read: invalid file descriptor %d", fd)
		}
		reader = e.inputFiles[fileIndex]
	}

	// Read data
	buffer := make([]byte, count)
	n, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		e.stats.ErrorCount++
		return "", fmt.Errorf("read: %w", err)
	}

	e.stats.BytesRead += int64(n)
	return string(buffer[:n]), nil
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

// executePipe implements the pipe tool (placeholder)
func (e *Engine) executePipe(args map[string]interface{}) (string, error) {
	e.stats.PipeCalls++

	// Extract command
	command, ok := args["command"].(string)
	if !ok {
		e.stats.ErrorCount++
		return "", fmt.Errorf("pipe: command parameter must be a string")
	}

	// TODO: Phase 4 - Implement built-in commands
	// For now, return a placeholder
	return fmt.Sprintf("pipe: %s command not yet implemented", command), nil
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

	os.Exit(code)
	return "", nil // Never reached
}

// GetStats returns current execution statistics
func (e *Engine) GetStats() ExecutionStats {
	return e.stats
}
