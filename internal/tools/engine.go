package tools

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/mako10k/llmcmd/internal/tools/builtin"
)

// isBinaryFile checks if a file is binary by examining its extension and content
func isBinaryFile(filename string) bool {
	// Check common binary file extensions
	ext := strings.ToLower(filepath.Ext(filename))
	binaryExts := []string{
		".exe", ".dll", ".so", ".dylib", ".a", ".o", ".obj",
		".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar",
		".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg", ".ico",
		".mp3", ".wav", ".ogg", ".flac", ".aac", ".wma",
		".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv",
		".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
		".bin", ".iso", ".img", ".dmg",
	}

	for _, binaryExt := range binaryExts {
		if ext == binaryExt {
			return true
		}
	}

	// Check file content for binary data
	file, err := os.Open(filename)
	if err != nil {
		// If we can't open it, assume it's text and let the error be handled later
		return false
	}
	defer file.Close()

	// Read first 512 bytes to check for binary content
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return false
	}

	// Check for null bytes or high percentage of non-printable characters
	nullBytes := 0
	nonPrintable := 0
	for i := 0; i < n; i++ {
		if buffer[i] == 0 {
			nullBytes++
		}
		if buffer[i] < 32 && buffer[i] != 9 && buffer[i] != 10 && buffer[i] != 13 {
			nonPrintable++
		}
	}

	// If more than 30% non-printable characters or any null bytes, consider binary
	if nullBytes > 0 || (float64(nonPrintable)/float64(n)) > 0.30 {
		return true
	}

	return false
}

// RunningCommand tracks a running command and its pipes
type RunningCommand struct {
	cmd      *exec.Cmd
	stdout   io.ReadCloser
	stderr   io.ReadCloser
	stdin    io.WriteCloser
	done     chan error
	exitCode int
	finished bool
	mu       sync.RWMutex

	// File descriptor mappings for this command
	inputFd     int    // The fd this command reads from
	outputFd    int    // The fd this command writes to
	pid         int    // Process ID
	commandName string // Command name for debugging
}

// FdDependency represents a file descriptor dependency relationship
type FdDependency struct {
	Source   int    // Source fd (input)
	Targets  []int  // Target fds (outputs) - supports 1:many for tee
	ToolType string // "spawn" or "tee"
}

// Engine handles tool execution for llmcmd
type Engine struct {
	inputFiles      []*os.File
	outputFile      *os.File
	fileDescriptors []interface{} // Can hold io.Reader, io.Writer, or io.ReadWriter
	runningCommands map[int]*RunningCommand // Maps fd to running command
	commandsMutex   sync.RWMutex
	fdDependencies  []FdDependency // Tracks fd dependencies for spawns and tees
	closedFds       map[int]bool   // Tracks which fds have been closed
	chainMutex      sync.RWMutex   // Protects fdDependencies and closedFds
	nextFd          int            // Next available file descriptor number
	maxFileSize     int64
	bufferSize      int
	stats           ExecutionStats
	noStdin         bool // Skip reading from stdin
}

// ExecutionStats tracks tool execution statistics
type ExecutionStats struct {
	ReadCalls    int   `json:"read_calls"`
	WriteCalls   int   `json:"write_calls"`
	SpawnCalls   int   `json:"spawn_calls"`
	TeeCalls     int   `json:"tee_calls"`
	CloseCalls   int   `json:"close_calls"`
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
}

// NewEngine creates a new tool execution engine
func NewEngine(config EngineConfig) (*Engine, error) {
	engine := &Engine{
		maxFileSize:     config.MaxFileSize,
		bufferSize:      config.BufferSize,
		noStdin:         config.NoStdin,
		runningCommands: make(map[int]*RunningCommand),
		fdDependencies:  []FdDependency{},
		closedFds:       make(map[int]bool),
		nextFd:          10, // Start at 10, reserving 0-9 for standard fds
	}

	// Initialize file descriptors array
	// 0=stdin, 1=stdout, 2=stderr, 3+=input files
	engine.fileDescriptors = make([]interface{}, 3)
	if !config.NoStdin {
		engine.fileDescriptors[0] = os.Stdin
	}
	// Add stdout and stderr to fd management
	if engine.outputFile != nil {
		engine.fileDescriptors[1] = engine.outputFile
	} else {
		engine.fileDescriptors[1] = os.Stdout
	}
	engine.fileDescriptors[2] = os.Stderr

	// Open input files and add to file descriptors
	for _, filename := range config.InputFiles {
		if filename == "-" {
			// "-" means stdin, so add stdin as an additional file descriptor
			engine.fileDescriptors = append(engine.fileDescriptors, os.Stdin)
		} else {
			// Check if file is binary before opening
			if isBinaryFile(filename) {
				return nil, fmt.Errorf("binary file detected: %s - llmcmd only supports text files for security and cost reasons", filename)
			}

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

// addFdDependency adds a new file descriptor dependency relationship
func (e *Engine) addFdDependency(source int, targets []int, toolType string) {
	e.chainMutex.Lock()
	defer e.chainMutex.Unlock()

	dependency := FdDependency{
		Source:   source,
		Targets:  targets,
		ToolType: toolType,
	}
	e.fdDependencies = append(e.fdDependencies, dependency)
}

// checkCloseDependencies checks if an input fd can be closed based on dependency rules
// markFdClosed marks a file descriptor as closed
func (e *Engine) markFdClosed(fd int) {
	e.chainMutex.Lock()
	defer e.chainMutex.Unlock()
	e.closedFds[fd] = true
}

// traverseChainOnEOF traverses the chain when EOF is detected and collects exit codes
func (e *Engine) traverseChainOnEOF(startFd int) []ChainResult {
	e.chainMutex.RLock()
	defer e.chainMutex.RUnlock()

	var results []ChainResult
	visited := make(map[int]bool)

	e.traverseChainRecursive(startFd, visited, &results)
	return results
}

// ChainResult represents the result of a command in the chain
type ChainResult struct {
	Fd       int    `json:"fd"`
	ExitCode int    `json:"exit_code"`
	Command  string `json:"command"`
	Message  string `json:"message"`
}

// traverseChainRecursive recursively traverses the dependency chain
func (e *Engine) traverseChainRecursive(fd int, visited map[int]bool, results *[]ChainResult) {
	if visited[fd] {
		return // Avoid infinite loops
	}
	visited[fd] = true

	// Special case: STDIN (fd=0)
	if fd == 0 {
		*results = append(*results, ChainResult{
			Fd:       0,
			ExitCode: 0,
			Command:  "stdin",
			Message:  "Chain traversal reached STDIN (fd=0) - chain root found",
		})
		return
	}

	// Find dependencies where this fd is a target (reverse lookup)
	for _, dep := range e.fdDependencies {
		for _, targetFd := range dep.Targets {
			if targetFd == fd {
				// Found upstream dependency, get command info and exit code
				var result ChainResult
				result.Fd = dep.Source

				// Get command information
				e.commandsMutex.RLock()
				if runningCmd, exists := e.runningCommands[dep.Source]; exists {
					runningCmd.mu.RLock()
					result.ExitCode = runningCmd.exitCode
					result.Command = runningCmd.commandName
					result.Message = fmt.Sprintf("Command '%s' on fd %d exited with code %d",
						runningCmd.commandName, dep.Source, runningCmd.exitCode)
					runningCmd.mu.RUnlock()
				} else {
					result.ExitCode = 0
					result.Command = "unknown"
					result.Message = fmt.Sprintf("No command information for fd %d", dep.Source)
				}
				e.commandsMutex.RUnlock()

				*results = append(*results, result)

				// Continue traversing upstream
				e.traverseChainRecursive(dep.Source, visited, results)
			}
		}
	}
}

// allocateFd allocates a new file descriptor number
func (e *Engine) allocateFd() int {
	e.chainMutex.Lock()
	defer e.chainMutex.Unlock()
	fd := e.nextFd
	e.nextFd++
	return fd
}

// spawnError creates a standardized spawn error with stats increment
func (e *Engine) spawnError(message string, err error) (string, error) {
	e.stats.ErrorCount++
	return "", fmt.Errorf("spawn: %s: %w", message, err)
}

// spawnSuccess creates a standardized spawn success result
func (e *Engine) spawnSuccess(result map[string]interface{}) (string, error) {
	resultBytes, _ := json.Marshal(result)
	return string(resultBytes), nil
}

// createRunningCommand creates and stores a new RunningCommand
func (e *Engine) createRunningCommand(cmd string, args []string, fd int, inputFd, outputFd int, stdin io.WriteCloser, stdout io.ReadCloser) *RunningCommand {
	runningCmd := &RunningCommand{
		stdin:       stdin,
		stdout:      stdout,
		done:        make(chan error, 1),
		inputFd:     inputFd,
		outputFd:    outputFd,
		pid:         fd, // Use fd as pseudo-pid
		commandName: fmt.Sprintf("%s %v", cmd, args),
	}

	e.commandsMutex.Lock()
	e.runningCommands[fd] = runningCmd
	e.commandsMutex.Unlock()

	return runningCmd
}

// startBackgroundCommand starts a built-in command in the background and returns file descriptors
func (e *Engine) startBackgroundCommand(cmd string, args []string) (int, int, error) {
	// Create pipes for communication
	inReader, inWriter, err := os.Pipe()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create input pipe: %w", err)
	}

	outReader, outWriter, err := os.Pipe()
	if err != nil {
		inReader.Close()
		inWriter.Close()
		return 0, 0, fmt.Errorf("failed to create output pipe: %w", err)
	}

	// Allocate file descriptors
	inFd := e.allocateFd()
	outFd := e.allocateFd()

	// Create running command tracker
	runningCmd := &RunningCommand{
		stdin:       inWriter,
		stdout:      outReader,
		done:        make(chan error, 1),
		inputFd:     inFd,
		outputFd:    outFd,
		pid:         inFd, // Use fd as pseudo-pid for built-in commands
		commandName: fmt.Sprintf("%s %v", cmd, args),
	}

	// Store the command
	e.commandsMutex.Lock()
	e.runningCommands[inFd] = runningCmd
	e.runningCommands[outFd] = runningCmd
	e.commandsMutex.Unlock()

	// Extend file descriptors array if needed
	for len(e.fileDescriptors) <= outFd {
		e.fileDescriptors = append(e.fileDescriptors, nil)
	}

	// Set up file descriptors for reading/writing
	e.fileDescriptors[outFd] = outReader // For reading command output

	// Start goroutine to execute built-in command
	go func() {
		defer func() {
			// Close pipes when command finishes
			inReader.Close()
			outWriter.Close()

			runningCmd.mu.Lock()
			runningCmd.finished = true
			runningCmd.mu.Unlock()

			runningCmd.done <- nil
			close(runningCmd.done)
		}()

		// Execute the built-in command
		var err error
		commandFunc, exists := builtin.Commands[cmd]
		if !exists {
			err = fmt.Errorf("unknown command: %s", cmd)
		} else {
			err = commandFunc(args, inReader, outWriter)
		}

		runningCmd.mu.Lock()
		if err != nil {
			runningCmd.exitCode = 1
		} else {
			runningCmd.exitCode = 0
		}
		runningCmd.mu.Unlock()
	}()

	return inFd, outFd, nil
}

// startBackgroundCommandWithInput starts a command that reads from existing in_fd
func (e *Engine) startBackgroundCommandWithInput(cmd string, args []string, inputFd int, size int) (int, error) {
	// Validate input file descriptor
	if inputFd < 0 || inputFd >= len(e.fileDescriptors) || e.fileDescriptors[inputFd] == nil {
		return 0, fmt.Errorf("invalid input file descriptor: %d", inputFd)
	}

	// Create output pipe
	outReader, outWriter, err := os.Pipe()
	if err != nil {
		return 0, fmt.Errorf("failed to create output pipe: %w", err)
	}

	// Allocate output file descriptor
	outFd := e.allocateFd()

	// Create and store running command tracker
	runningCmd := e.createRunningCommand(cmd, args, outFd, inputFd, outFd, nil, outReader)

	// Extend file descriptors array if needed
	for len(e.fileDescriptors) <= outFd {
		e.fileDescriptors = append(e.fileDescriptors, nil)
	}

	// Set up file descriptor for reading command output
	e.fileDescriptors[outFd] = outReader

	// Start goroutine to execute built-in command
	go func() {
		defer func() {
			outWriter.Close()

			runningCmd.mu.Lock()
			runningCmd.finished = true
			runningCmd.mu.Unlock()

			runningCmd.done <- nil
			close(runningCmd.done)
		}()

		// Read limited input data
		var inputData []byte
		if size > 0 {
			buf := make([]byte, size)
			reader, ok := e.fileDescriptors[inputFd].(io.Reader)
			if !ok {
				runningCmd.mu.Lock()
				runningCmd.exitCode = 1
				runningCmd.mu.Unlock()
				return
			}
			n, err := reader.Read(buf)
			if err != nil && err != io.EOF {
				runningCmd.mu.Lock()
				runningCmd.exitCode = 1
				runningCmd.mu.Unlock()
				return
			}
			inputData = buf[:n]
		}

		// Execute the built-in command
		var err error
		inReader := bytes.NewReader(inputData)

		commandFunc, exists := builtin.Commands[cmd]
		if !exists {
			err = fmt.Errorf("unknown command: %s", cmd)
		} else {
			err = commandFunc(args, inReader, outWriter)
		}

		runningCmd.mu.Lock()
		if err != nil {
			runningCmd.exitCode = 1
		} else {
			runningCmd.exitCode = 0
		}
		runningCmd.mu.Unlock()
	}()

	return outFd, nil
}

// startBackgroundCommandWithExistingInput starts a command that reads from existing in_fd (reads all available data)
func (e *Engine) startBackgroundCommandWithExistingInput(cmd string, args []string, inputFd int) (int, error) {
	// Validate input file descriptor
	if inputFd < 0 || inputFd >= len(e.fileDescriptors) || e.fileDescriptors[inputFd] == nil {
		return 0, fmt.Errorf("invalid input file descriptor: %d", inputFd)
	}

	// Create output pipe
	outReader, outWriter, err := os.Pipe()
	if err != nil {
		return 0, fmt.Errorf("failed to create output pipe: %w", err)
	}

	// Allocate output file descriptor
	outFd := e.allocateFd()

	// Create and store running command tracker
	runningCmd := e.createRunningCommand(cmd, args, outFd, inputFd, outFd, nil, outReader)

	// Extend file descriptors array if needed
	for len(e.fileDescriptors) <= outFd {
		e.fileDescriptors = append(e.fileDescriptors, nil)
	}

	// Set up file descriptor for reading command output
	e.fileDescriptors[outFd] = outReader

	// Start goroutine to execute built-in command
	go func() {
		defer func() {
			outWriter.Close()

			runningCmd.mu.Lock()
			runningCmd.finished = true
			runningCmd.mu.Unlock()

			runningCmd.done <- nil
			close(runningCmd.done)
		}()

		// Execute the built-in command directly with input stream
		commandFunc, exists := builtin.Commands[cmd]
		if !exists {
			runningCmd.mu.Lock()
			runningCmd.exitCode = 1
			runningCmd.mu.Unlock()
			return
		}

		reader, ok := e.fileDescriptors[inputFd].(io.Reader)
		if !ok {
			runningCmd.mu.Lock()
			runningCmd.exitCode = 1
			runningCmd.mu.Unlock()
			return
		}

		err := commandFunc(args, reader, outWriter)
		runningCmd.mu.Lock()
		if err != nil {
			runningCmd.exitCode = 1
		} else {
			runningCmd.exitCode = 0
		}
		runningCmd.mu.Unlock()
	}()

	return outFd, nil
}

// startBackgroundCommandWithInputOutput starts a command that reads from in_fd and creates a new output fd (pipe chain middle)
// startBackgroundCommandWithInputOutput starts a command that reads from in_fd and writes to out_fd (pipe chain middle)
func (e *Engine) startBackgroundCommandWithInputOutput(cmd string, args []string, inputFd int) error {
	// Validate input file descriptor
	if inputFd < 0 || inputFd >= len(e.fileDescriptors) || e.fileDescriptors[inputFd] == nil {
		return fmt.Errorf("invalid input file descriptor: %d", inputFd)
	}

	// Writing to arbitrary file descriptor not yet implemented - fd management redesign needed
	return fmt.Errorf("startBackgroundCommandWithInputOutput not yet implemented - fd management redesign needed")
}

// startBackgroundCommandWithOutput starts a command that writes to existing out_fd
func (e *Engine) startBackgroundCommandWithOutput(cmd string, args []string, outputFd int) (int, error) {
	// Validate output file descriptor exists
	if outputFd < 0 || outputFd >= len(e.fileDescriptors) || e.fileDescriptors[outputFd] == nil {
		return 0, fmt.Errorf("invalid output file descriptor: %d", outputFd)
	}

	// Writing to arbitrary file descriptor not yet implemented - fd management redesign needed
	return 0, fmt.Errorf("writing to arbitrary file descriptor %d not yet implemented - fd management redesign needed", outputFd)
}

// Close closes all file handles
func (e *Engine) Close() error {
	var errors []error

	// Close file descriptors (skip fd 0 as it's managed by the parent process)
	for i, fdObj := range e.fileDescriptors {
		if i == 0 {
			// Skip stdin (fd 0) - managed by parent process
			continue
		}
		if fdObj != nil {
			if closer, ok := fdObj.(io.Closer); ok {
				if err := closer.Close(); err != nil {
					errors = append(errors, fmt.Errorf("error closing fd %d: %w", i, err))
				}
			}
		}
	}

	// Close input files (these might overlap with fileDescriptors, but Close() is idempotent)
	for _, file := range e.inputFiles {
		if err := file.Close(); err != nil {
			errors = append(errors, err)
		}
	}

	// Close output file (this might overlap with fd 1, but Close() is idempotent)  
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
	case "spawn":
		return e.executeSpawn(args)
	case "tee":
		return e.executeTee(args)
	case "close":
		return e.executeClose(args)
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

	// Check for lines parameter (alternative to count)
	if linesFloat, hasLines := args["lines"].(float64); hasLines {
		lines := int(linesFloat)
		if lines <= 0 || lines > 1000 {
			e.stats.ErrorCount++
			return "", fmt.Errorf("read: lines must be between 1 and 1000")
		}
		return e.readLines(fd, lines)
	}

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

	fdObj := e.fileDescriptors[fd]
	if fdObj == nil {
		e.stats.ErrorCount++
		return "", fmt.Errorf("read: file descriptor %d not available", fd)
	}

	var readerOk bool
	reader, readerOk = fdObj.(io.Reader)
	if !readerOk {
		e.stats.ErrorCount++
		return "", fmt.Errorf("read: file descriptor %d is not readable", fd)
	}

	// Read data with blocking I/O
	buffer := make([]byte, count)
	n, err := reader.Read(buffer)
	
	// Handle all possible outcomes explicitly (Fail-First principle)
	if err != nil {
		if err == io.EOF {
			// EOF is a normal termination condition - report it clearly
			e.stats.BytesRead += int64(n)
			if n > 0 {
				// Return partial data with EOF indication
				return fmt.Sprintf("%s\n--- EOF reached after %d bytes ---", string(buffer[:n]), n), nil
			} else {
				// Pure EOF with no data
				return "--- EOF: No more data available ---", nil
			}
		} else {
			// All other errors are failures (Fail-First)
			e.stats.ErrorCount++
			return "", fmt.Errorf("read: %w", err)
		}
	}

	e.stats.BytesRead += int64(n)
	result := string(buffer[:n])

	// Contract: Always return clear information about what was read
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

	// Extract newline parameter (optional, default false)
	addNewline := false
	if newlineVal, ok := args["newline"].(bool); ok {
		addNewline = newlineVal
	}

	// Extract eof parameter (optional, default false)
	isEof := false
	if eofVal, ok := args["eof"].(bool); ok {
		isEof = eofVal
	}

	// Get the appropriate writer
	var writer io.Writer

	// First check if it's a special fd (0-2) from fileDescriptors
	if fd >= 0 && fd < len(e.fileDescriptors) && e.fileDescriptors[fd] != nil {
		if w, ok := e.fileDescriptors[fd].(io.Writer); ok {
			writer = w
		} else {
			e.stats.ErrorCount++
			return "", fmt.Errorf("write: file descriptor %d is not writable", fd)
		}
	} else {
		// Check if this is a running command's input fd
		e.commandsMutex.RLock()
		if runningCmd, exists := e.runningCommands[fd]; exists {
			if runningCmd.inputFd == fd && runningCmd.stdin != nil {
				writer = runningCmd.stdin
				e.commandsMutex.RUnlock()
			} else {
				e.commandsMutex.RUnlock()
				e.stats.ErrorCount++
				return "", fmt.Errorf("write: fd %d is not an input fd for a running command", fd)
			}
		} else {
			e.commandsMutex.RUnlock()
			e.stats.ErrorCount++
			return "", fmt.Errorf("write: invalid file descriptor %d", fd)
		}
	}

	// Add newline if requested
	if addNewline {
		data += "\n"
	}

	// Write data
	n, err := writer.Write([]byte(data))
	if err != nil {
		e.stats.ErrorCount++
		return "", fmt.Errorf("write: %w", err)
	}

	e.stats.BytesWritten += int64(n)

	// Handle EOF - trigger chain cleanup if eof is true
	if isEof {
		if fd >= 3 {
			// Pipeline intermediate (fd 3+): auto-close on EOF
			if closer, ok := writer.(io.Closer); ok {
				closer.Close()
			}
			// Mark FD as closed and trigger chain processing
			e.markFdClosed(fd)
		} else {
			// Pipeline endpoints (fd 0,1,2): EOF is just a marker, explicit close needed
			// Don't mark as closed - explicit close tool required for actual closing
		}

		// Traverse the chain to collect exit codes (for all fds)
		chainResults := e.traverseChainOnEOF(fd)

		// Create summary message
		var summary strings.Builder
		if fd >= 3 {
			summary.WriteString(fmt.Sprintf("wrote %d bytes to fd %d (EOF), auto-closed, chain traversal results:\n", n, fd))
		} else {
			summary.WriteString(fmt.Sprintf("wrote %d bytes to fd %d (EOF), explicit close required, chain traversal results:\n", n, fd))
		}
		for _, result := range chainResults {
			summary.WriteString(fmt.Sprintf("  %s\n", result.Message))
		}

		return summary.String(), nil
	}

	return fmt.Sprintf("wrote %d bytes to fd %d", n, fd), nil
}

// executeSpawn implements the spawn tool for executing built-in commands
func (e *Engine) executeSpawn(args map[string]interface{}) (string, error) {
	e.stats.SpawnCalls++

	// Extract command name (required)
	cmd, ok := args["cmd"].(string)
	if !ok {
		e.stats.ErrorCount++
		return "", fmt.Errorf("spawn: cmd parameter is required")
	}

	// Validate that the command is a built-in command
	if _, exists := builtin.Commands[cmd]; !exists {
		e.stats.ErrorCount++
		return "", fmt.Errorf("spawn: command '%s' is not a supported built-in command. Available commands: %v", 
			cmd, getSupportedCommands())
	}

	// Extract command arguments (optional)
	var cmdArgs []string
	if argsInterface, hasArgs := args["args"]; hasArgs {
		if argsList, ok := argsInterface.([]interface{}); ok {
			for _, arg := range argsList {
				if argStr, ok := arg.(string); ok {
					cmdArgs = append(cmdArgs, argStr)
				}
			}
		}
	}

	// Extract optional parameters
	var inFd *int
	var outFd *int

	if inFdFloat, hasInFd := args["in_fd"].(float64); hasInFd {
		inFdInt := int(inFdFloat)
		inFd = &inFdInt
	}

	if outFdFloat, hasOutFd := args["out_fd"].(float64); hasOutFd {
		outFdInt := int(outFdFloat)
		outFd = &outFdInt
	}

	// Determine execution pattern based on arguments
	result := map[string]interface{}{
		"success": true,
	}

	// Pattern 1: spawn({cmd:...,args:...}) -> {success:true,in_fd:..., out_fd:...}
	// Background execution, return file descriptors
	if inFd == nil && outFd == nil {
		// Start background command with real pipes
		realInFd, realOutFd, err := e.startBackgroundCommand(cmd, cmdArgs)
		if err != nil {
			return e.spawnError("failed to start background command", err)
		}

		// Record the dependency relationship
		e.addFdDependency(realInFd, []int{realOutFd}, "spawn")

		result["in_fd"] = realInFd
		result["out_fd"] = realOutFd
		return e.spawnSuccess(result)
	}

	// Pattern 2: spawn({cmd:...,args:...,in_fd:...}) -> {success:true,out_fd:...}
	// Background execution with input from existing in_fd
	if inFd != nil && outFd == nil {
		// Start background command that reads from existing in_fd
		realOutFd, err := e.startBackgroundCommandWithExistingInput(cmd, cmdArgs, *inFd)
		if err != nil {
			return e.spawnError("failed to start background command with existing input", err)
		}

		result["out_fd"] = realOutFd
		return e.spawnSuccess(result)
	}

	// Pattern 3: spawn({cmd:...,args:...,out_fd:...}) -> {success:true,in_fd:...}
	// Background execution with output to out_fd
	if inFd == nil && outFd != nil {
		// Start background command that writes to existing out_fd
		realInFd, err := e.startBackgroundCommandWithOutput(cmd, cmdArgs, *outFd)
		if err != nil {
			return e.spawnError("failed to start background command with output", err)
		}

		result["in_fd"] = realInFd
		return e.spawnSuccess(result)
	}

	// Pattern 4: spawn({cmd:...,args:...,in_fd:...,out_fd:...}) -> {success:true}
	// Background execution with input from in_fd and output to out_fd (pipe chain middle processing)
	if inFd != nil && outFd != nil {
		err := e.startBackgroundCommandWithInputOutput(cmd, cmdArgs, *inFd)
		if err != nil {
			return e.spawnError("failed to start background command with input and output", err)
		}

		return e.spawnSuccess(result)
	}

	// Should not reach here
	return e.spawnError("no valid spawn pattern matched", fmt.Errorf("invalid spawn arguments"))
}

// executeBuiltinCommand executes a single built-in command
func (e *Engine) executeBuiltinCommand(cmd string, args []string, input []byte) ([]byte, error) {
	commandFunc, exists := builtin.Commands[cmd]
	if !exists {
		return nil, fmt.Errorf("unsupported command: %s", cmd)
	}

	// Prepare input and output
	inputReader := bytes.NewReader(input)
	var output bytes.Buffer

	// Execute command
	if err := commandFunc(args, inputReader, &output); err != nil {
		return nil, fmt.Errorf("command '%s' failed: %w", cmd, err)
	}

	return output.Bytes(), nil
}

// executeClose implements the close tool - explicitly closes file descriptors
func (e *Engine) executeClose(args map[string]interface{}) (string, error) {
	e.stats.CloseCalls++

	// Extract file descriptor
	fdFloat, ok := args["fd"].(float64)
	if !ok {
		e.stats.ErrorCount++
		return "", fmt.Errorf("close: fd parameter must be a number")
	}
	fd := int(fdFloat)

	// Validate file descriptor
	if fd < 0 || fd >= len(e.fileDescriptors) || e.fileDescriptors[fd] == nil {
		e.stats.ErrorCount++
		return "", fmt.Errorf("close: invalid file descriptor %d", fd)
	}

	// Check if already closed
	e.chainMutex.RLock()
	if e.closedFds[fd] {
		e.chainMutex.RUnlock()
		e.stats.ErrorCount++
		return "", fmt.Errorf("close: file descriptor %d is already closed", fd)
	}
	e.chainMutex.RUnlock()

	// Perform the close operation
	fdObj := e.fileDescriptors[fd]
	if closer, ok := fdObj.(io.Closer); ok {
		if fd < 3 {
			// Pipeline endpoints (0,1,2): explicit close for flush and EOF notification
			if err := closer.Close(); err != nil {
				e.stats.ErrorCount++
				return "", fmt.Errorf("close: error closing fd %d: %w", fd, err)
			}
		} else {
			// Internal fds (3+): should already be auto-closed, but allow explicit close
			if err := closer.Close(); err != nil {
				e.stats.ErrorCount++
				return "", fmt.Errorf("close: error closing fd %d: %w", fd, err)
			}
		}
	}

	// Mark as closed and trigger chain processing
	e.markFdClosed(fd)

	// Traverse the chain to collect exit codes
	chainResults := e.traverseChainOnEOF(fd)

	// Create summary message
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("closed fd %d, chain traversal results:\n", fd))
	for _, result := range chainResults {
		summary.WriteString(fmt.Sprintf("  fd %d: %s (exit: %d, cmd: %s)\n", 
			result.Fd, result.Message, result.ExitCode, result.Command))
	}

	return summary.String(), nil
}

// getSupportedCommands returns a sorted list of supported built-in commands
func getSupportedCommands() []string {
	var commands []string
	for cmd := range builtin.Commands {
		commands = append(commands, cmd)
	}
	sort.Strings(commands)
	return commands
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

// executeTee implements the tee tool for copying input to multiple outputs
func (e *Engine) executeTee(args map[string]interface{}) (string, error) {
	e.stats.TeeCalls++

	// Extract input file descriptor
	inFdFloat, ok := args["in_fd"].(float64)
	if !ok {
		e.stats.ErrorCount++
		return "", fmt.Errorf("tee: in_fd parameter must be a number")
	}
	inFd := int(inFdFloat)

	// Extract output file descriptors array
	outFdsInterface, ok := args["out_fds"].([]interface{})
	if !ok {
		e.stats.ErrorCount++
		return "", fmt.Errorf("tee: out_fds parameter must be an array")
	}

	var outFds []int
	for _, fdInterface := range outFdsInterface {
		if fdFloat, ok := fdInterface.(float64); ok {
			outFds = append(outFds, int(fdFloat))
		} else {
			e.stats.ErrorCount++
			return "", fmt.Errorf("tee: all out_fds elements must be numbers")
		}
	}

	if len(outFds) == 0 {
		e.stats.ErrorCount++
		return "", fmt.Errorf("tee: at least one output fd required")
	}

	// Record the dependency relationship for tee (1:many)
	e.addFdDependency(inFd, outFds, "tee")

	// Validate and get reader for input fd
	if inFd < 0 || inFd >= len(e.fileDescriptors) || e.fileDescriptors[inFd] == nil {
		e.stats.ErrorCount++
		return "", fmt.Errorf("tee: invalid input file descriptor: %d", inFd)
	}

	reader, readerOk := e.fileDescriptors[inFd].(io.Reader)
	if !readerOk {
		e.stats.ErrorCount++
		return "", fmt.Errorf("tee: file descriptor %d is not readable", inFd)
	}

	// Read all input data
	var inputData []byte
	buf := make([]byte, 4096)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			inputData = append(inputData, buf[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			e.stats.ErrorCount++
			return "", fmt.Errorf("tee: failed to read from fd %d: %w", inFd, err)
		}
	}

	// Write to all output file descriptors
	totalWritten := 0
	for _, outFd := range outFds {
		if outFd >= 0 && outFd < len(e.fileDescriptors) && e.fileDescriptors[outFd] != nil {
			if writer, ok := e.fileDescriptors[outFd].(io.Writer); ok {
				n, _ := writer.Write(inputData)
				totalWritten += n
			}
		}
	}

	e.stats.BytesRead += int64(len(inputData))
	e.stats.BytesWritten += int64(totalWritten)

	return fmt.Sprintf("tee: copied %d bytes from fd %d to %d outputs", len(inputData), inFd, len(outFds)), nil
}

// GetStats returns current execution statistics
func (e *Engine) GetStats() ExecutionStats {
	return e.stats
}

// readLines reads a specified number of lines from a file descriptor
func (e *Engine) readLines(fd int, lines int) (string, error) {
	// Get the appropriate reader
	if fd < 0 || fd >= len(e.fileDescriptors) {
		e.stats.ErrorCount++
		return "", fmt.Errorf("read: invalid file descriptor %d", fd)
	}

	fdObj := e.fileDescriptors[fd]
	if fdObj == nil {
		e.stats.ErrorCount++
		return "", fmt.Errorf("read: file descriptor %d not available", fd)
	}

	reader, readerOk := fdObj.(io.Reader)
	if !readerOk {
		e.stats.ErrorCount++
		return "", fmt.Errorf("read: file descriptor %d is not readable", fd)
	}

	var result strings.Builder
	scanner := bufio.NewScanner(reader)
	lineCount := 0

	for scanner.Scan() && lineCount < lines {
		if lineCount > 0 {
			result.WriteString("\n")
		}
		result.WriteString(scanner.Text())
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		e.stats.ErrorCount++
		return "", fmt.Errorf("read: %w", err)
	}

	resultStr := result.String()
	e.stats.BytesRead += int64(len(resultStr))
	return resultStr, nil
}
