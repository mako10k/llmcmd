package tools

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mako10k/llmcmd/internal/utils"
)

// ShellExecutor interface for executing shell commands
type ShellExecutor interface {
	Execute(command string) error
	ExecuteWithIO(command string, stdin io.Reader, stdout, stderr io.Writer) error
	// SetVFS allows shell executor to use virtual file system for redirects
	SetVFS(vfs VirtualFileSystem)
}

// InternalShellRunner runs shell scripts inside the process using llmsh parser+executor
type InternalShellRunner struct{ nextPID *int64 }

func NewInternalShellRunner(pidCounter *int64) *InternalShellRunner {
	return &InternalShellRunner{nextPID: pidCounter}
}

type InternalRunResult struct {
	PID      int
	Err      error
	ExitCode int
}

// RunScript executes script synchronously writing output/errors to provided writers.
func (r *InternalShellRunner) RunScript(script string, stdin io.Reader, stdout, stderr io.Writer) InternalRunResult {
	pid := int(atomic.AddInt64(r.nextPID, 1))
	start := time.Now()
	var err error
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("panic: %v", rec)
		}
	}()
	// TODO(feature/spawn-internal): Replace naive echo with real llmsh parser+executor pipeline.
	// Current behavior: simply writes script text followed by newline to stdout; ignores stdin.
	if _, werr := stdout.Write([]byte(script + "\n")); werr != nil {
		err = werr
	}
	_ = time.Since(start)
	code := 0
	if err != nil {
		code = 1
	}
	return InternalRunResult{PID: pid, Err: err, ExitCode: code}
}

// ToolSpec defines contract for a tool's accepted / required / forbidden parameters
type ToolSpec struct {
	Required  []string
	Optional  []string
	Forbidden []string
	// AllowUnknown if true suppresses error on unknown keys (legacy);
	// we keep false for strict fail-first.
	AllowUnknown bool
}

var toolSpecs = map[string]ToolSpec{
	"read":  {Required: []string{"fd"}, Optional: []string{"count", "lines"}},
	"write": {Required: []string{"fd", "data"}, Optional: []string{"newline", "eof"}},
	"spawn": {Required: []string{"script"}, Optional: []string{"stdin_fd", "stdout_fd"}, Forbidden: []string{"in_fd", "out_fd"}},
	"close": {Required: []string{"fd"}},
	"exit":  {Required: []string{"code"}, Optional: []string{"message"}},
	"open":  {Required: []string{"path"}, Optional: []string{"mode"}},
	"help":  {Optional: []string{"keys"}},
}

// validateArgs enforces the ToolSpec for a tool before execution.
func validateArgs(toolName string, args map[string]interface{}) error {
	spec, ok := toolSpecs[toolName]
	if !ok {
		// Unknown tool spec: treat as internal error to avoid silent acceptance
		return fmt.Errorf("validation: no spec for tool %s", toolName)
	}
	// Required check
	for _, r := range spec.Required {
		if _, present := args[r]; !present {
			return fmt.Errorf("%s: missing required parameter '%s'", toolName, r)
		}
	}
	// Forbidden check
	for _, f := range spec.Forbidden {
		if _, present := args[f]; present {
			return fmt.Errorf("%s: deprecated / forbidden parameter '%s'", toolName, f)
		}
	}
	// Unknown detection (strict mode)
	if !spec.AllowUnknown {
		allowed := make(map[string]struct{})
		for _, r := range spec.Required {
			allowed[r] = struct{}{}
		}
		for _, o := range spec.Optional {
			allowed[o] = struct{}{}
		}
		for k := range args {
			if _, ok := allowed[k]; !ok {
				return fmt.Errorf("%s: unknown parameter '%s'", toolName, k)
			}
		}
	}
	return nil
}

// VirtualFileSystem interface for managing virtual files
type VirtualFileSystem interface {
	OpenFile(name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error)
	CreateTemp(pattern string) (io.ReadWriteCloser, string, error)
	RemoveFile(name string) error
	ListFiles() []string
}

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
	// Simplified placeholder for future richer tracking (kept minimal to avoid unused lints)
	commandName string
	// Process handle and completion signal
	cmd  *exec.Cmd
	done chan struct{}
}

// Engine handles tool execution for llmcmd
type Engine struct {
	inputFiles      []*os.File
	fileDescriptors []interface{}           // Can hold io.Reader, io.Writer, or io.ReadWriter
	runningCommands map[int]*RunningCommand // Maps fd to running command
	commandsMutex   sync.RWMutex
	// Engine-managed stdio bridge endpoints (now via mux)
	stdinPipeWriter  *io.PipeWriter // paired with fileDescriptors[0] (io.PipeReader)
	stdoutPipeReader *io.PipeReader // paired with fileDescriptors[1] (io.PipeWriter)
	stderrPipeReader *io.PipeReader // paired with fileDescriptors[2] (io.PipeWriter)
	stdioMux         *StdioMux
	// Track all running commands (even when no fd is returned, e.g., stdout_fd=1)
	runningAll []*RunningCommand
	procsMutex sync.RWMutex
	closedFds  map[int]bool // Tracks which fds have been closed
	chainMutex sync.RWMutex // Protects closedFds
	nextFd     int          // Next available file descriptor number
	bufferSize int
	stats      ExecutionStats
	// New components for llmsh integration
	virtualFS VirtualFileSystem
	// FD attach propagation for child llmsh
	vfsMuxFDs string
}

// ExecutionStats tracks tool execution statistics
type ExecutionStats struct {
	ReadCalls    int   `json:"read_calls"`
	WriteCalls   int   `json:"write_calls"`
	SpawnCalls   int   `json:"spawn_calls"`
	CloseCalls   int   `json:"close_calls"`
	ExitCalls    int   `json:"exit_calls"`
	BytesRead    int64 `json:"bytes_read"`
	BytesWritten int64 `json:"bytes_written"`
	ErrorCount   int   `json:"error_count"`
}

// EngineConfig holds configuration for the tool engine
type EngineConfig struct {
	InputFiles    []string
	OutputFiles   []string
	MaxFileSize   int64
	BufferSize    int
	NoStdin       bool // Skip reading from stdin
	ShellExecutor ShellExecutor
	VirtualFS     VirtualFileSystem
	// Optional attach FD spec to propagate to child llmsh ("fd" or "rfd,wfd")
	VFSMuxFDs string
}

// NewEngine creates a new tool execution engine
func NewEngine(config EngineConfig) (*Engine, error) {
	engine := &Engine{
		bufferSize:      config.BufferSize,
		runningCommands: make(map[int]*RunningCommand),
		runningAll:      make([]*RunningCommand, 0, 4),
		closedFds:       make(map[int]bool),
		nextFd:          10, // Start at 10, reserving 0-9 for standard fds
		virtualFS:       config.VirtualFS,
		vfsMuxFDs:       config.VFSMuxFDs,
	}

	// Initialize file descriptors array
	// 0=stdin, 1=stdout, 2=stderr, 3+=input files
	engine.fileDescriptors = make([]interface{}, 3)
	// STDIO via mux: create StdioMux and expose engine endpoints
	sm := NewStdioMux(engine.bufferSize)
	engine.stdioMux = sm
	// fd0 = stdin reader, fd1/2 = writers
	if config.NoStdin {
		pr, pw := io.Pipe()
		_ = pw.Close() // immediate EOF
		engine.fileDescriptors[0] = pr
		engine.stdinPipeWriter = nil
	} else {
		if r, ok := sm.StdinReader().(*io.PipeReader); ok {
			// Keep paired writer to allow Close on exit
			engine.fileDescriptors[0] = r
			// We don't have direct access to writer from StdinReader; use CloseStdinToEngine in exit
		} else {
			// Fallback minimal pipe EOF
			pr, pw := io.Pipe()
			_ = pw.Close()
			engine.fileDescriptors[0] = pr
		}
	}
	if w, ok := sm.StdoutWriter().(*io.PipeWriter); ok {
		engine.fileDescriptors[1] = w
	} else {
		pr, pw := io.Pipe()
		engine.fileDescriptors[1] = pw
		engine.stdoutPipeReader = pr
	}
	if w, ok := sm.StderrWriter().(*io.PipeWriter); ok {
		engine.fileDescriptors[2] = w
	} else {
		pr, pw := io.Pipe()
		engine.fileDescriptors[2] = pw
		engine.stderrPipeReader = pr
	}

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

	// Output files are handled through VirtualFS, not directly in engine

	return engine, nil
}

// addFdDependency adds a new file descriptor dependency relationship
// addFdDependency retained for potential future use (currently no-op to satisfy previous call sites removed)
// func (e *Engine) addFdDependency(source int, targets []int, toolType string) {}

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

	// Dependency tracking removed: no further traversal
}

// allocateFd allocates a new file descriptor number
func (e *Engine) allocateFd() int {
	e.chainMutex.Lock()
	defer e.chainMutex.Unlock()
	fd := e.nextFd
	e.nextFd++
	return fd
}

// Deprecated: spawnError legacy helper (unused)
// func (e *Engine) spawnError(message string, err error) (string, error) { e.stats.ErrorCount++; return "", fmt.Errorf("spawn: %s: %w", message, err) }

// spawnSuccess creates a standardized spawn success result
func (e *Engine) spawnSuccess(result map[string]interface{}) (string, error) {
	resultBytes, _ := json.Marshal(result)
	return string(resultBytes), nil
}

// Deprecated: createRunningCommand (internal background infra removed)
// func (e *Engine) createRunningCommand(cmd string, args []string, fd int, inputFd, outputFd int, stdin io.WriteCloser, stdout io.ReadCloser) *RunningCommand { return &RunningCommand{commandName: fmt.Sprintf("%s %v", cmd, args)} }

/*
// Deprecated background command API
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

	// Create running command tracker (minimal after refactor)
	runningCmd := &RunningCommand{commandName: fmt.Sprintf("%s %v", cmd, args)}

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

	// Start goroutine to execute built-in command (simplified)
	go func() {
		defer func() { inReader.Close(); outWriter.Close() }()
		commandFunc, exists := builtin.Commands[cmd]
		if !exists { return }
		_ = commandFunc(args, inReader, outWriter)
	}()

	return inFd, outFd, nil
}
*/

// startBackgroundCommandWithInput starts a command that reads from existing in_fd
// startBackgroundCommandWithInput deprecated
/*
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

	// Create tracker (minimal)
	// tracker removed in simplified internal runner

	// Extend file descriptors array if needed
	for len(e.fileDescriptors) <= outFd {
		e.fileDescriptors = append(e.fileDescriptors, nil)
	}

	// Set up file descriptor for reading command output
	e.fileDescriptors[outFd] = outReader

	go func() { defer outWriter.Close(); var inputData []byte; if size > 0 { buf := make([]byte, size); if reader, ok := e.fileDescriptors[inputFd].(io.Reader); ok { n, _ := reader.Read(buf); inputData = buf[:n] } }; inReader := bytes.NewReader(inputData); if commandFunc, ok := builtin.Commands[cmd]; ok { _ = commandFunc(args, inReader, outWriter) } }()

	return outFd, nil
}
*/

// startBackgroundCommandWithExistingInput starts a command that reads from existing in_fd (reads all available data)
// startBackgroundCommandWithExistingInput deprecated
/*
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

	// tracker removed in simplified internal runner

	// Extend file descriptors array if needed
	for len(e.fileDescriptors) <= outFd {
		e.fileDescriptors = append(e.fileDescriptors, nil)
	}

	// Set up file descriptor for reading command output
	e.fileDescriptors[outFd] = outReader

	go func() { defer outWriter.Close(); if commandFunc, ok := builtin.Commands[cmd]; ok { if reader, ok2 := e.fileDescriptors[inputFd].(io.Reader); ok2 { _ = commandFunc(args, reader, outWriter) } } }()

	return outFd, nil
}
*/

// startBackgroundCommandWithInputOutput starts a command that reads from in_fd and creates a new output fd (pipe chain middle)
// startBackgroundCommandWithInputOutput starts a command that reads from in_fd and writes to out_fd (pipe chain middle)
// startBackgroundCommandWithInputOutput deprecated
/*
func (e *Engine) startBackgroundCommandWithInputOutput(cmd string, args []string, inputFd int) error {
	// Validate input file descriptor
	if inputFd < 0 || inputFd >= len(e.fileDescriptors) || e.fileDescriptors[inputFd] == nil {
		return fmt.Errorf("invalid input file descriptor: %d", inputFd)
	}

	// Writing to arbitrary file descriptor not yet implemented - fd management redesign needed
	return fmt.Errorf("startBackgroundCommandWithInputOutput not yet implemented - fd management redesign needed")
}
*/

// startBackgroundCommandWithOutput starts a command that writes to existing out_fd
// startBackgroundCommandWithOutput deprecated
/*
func (e *Engine) startBackgroundCommandWithOutput(cmd string, args []string, outputFd int) (int, error) {
	// Validate output file descriptor exists
	if outputFd < 0 || outputFd >= len(e.fileDescriptors) || e.fileDescriptors[outputFd] == nil {
		return 0, fmt.Errorf("invalid output file descriptor: %d", outputFd)
	}

	// Writing to arbitrary file descriptor not yet implemented - fd management redesign needed
	return 0, fmt.Errorf("writing to arbitrary file descriptor %d not yet implemented - fd management redesign needed", outputFd)
}
*/

// Close closes all file handles
func (e *Engine) Close() error {
	var errors []error

	// Close file descriptors (engine-managed, including fd0/1/2 bridges)
	for i, fdObj := range e.fileDescriptors {
		if fdObj != nil {
			if closer, ok := fdObj.(io.Closer); ok {
				if err := closer.Close(); err != nil {
					errors = append(errors, fmt.Errorf("error closing fd %d: %w", i, err))
				}
			}
		}
	}

	if e.stdioMux != nil {
		_ = e.stdioMux.Close()
	}

	// Close input files (these might overlap with fileDescriptors, but Close() is idempotent)
	for _, file := range e.inputFiles {
		if err := file.Close(); err != nil {
			errors = append(errors, err)
		}
	}

	// No dedicated output file to close; stdout/stderr are managed by the parent process

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

	// Pre-validate arguments using common spec (fail-first)
	if err := validateArgs(functionName, args); err != nil {
		e.stats.ErrorCount++
		return "", err
	}

	// Execute the appropriate function
	switch functionName {
	case "read":
		return e.executeRead(args)
	case "write":
		return e.executeWrite(args)
	case "open":
		return e.executeOpen(args)
	case "spawn":
		return e.executeSpawn(args)
	case "close":
		return e.executeClose(args)
	case "exit":
		return e.executeExit(args)
	case "help":
		return e.executeHelp(args)
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
		if _, exists := e.runningCommands[fd]; exists {
			// Internal runner does not expose stdin for writing after spawn
			e.commandsMutex.RUnlock()
			e.stats.ErrorCount++
			return "", fmt.Errorf("write: stdin piping to spawned scripts not supported in current internal runner")
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

// executeSpawn implements the spawn tool using the shell executor
func (e *Engine) executeSpawn(args map[string]interface{}) (string, error) {
	e.stats.SpawnCalls++

	// Validate and extract script
	script, ok := args["script"].(string)
	if !ok {
		e.stats.ErrorCount++
		return "", fmt.Errorf("spawn: script parameter is required")
	}
	if strings.TrimSpace(script) == "" {
		e.stats.ErrorCount++
		return "", fmt.Errorf("spawn: script cannot be empty")
	}

	// Fail-fast for deprecated keys (already enforced by validateArgs but belt+suspenders)
	if _, hasOld := args["in_fd"]; hasOld {
		e.stats.ErrorCount++
		return "", fmt.Errorf("spawn: deprecated / forbidden parameter 'in_fd'")
	}
	if _, hasOld := args["out_fd"]; hasOld {
		e.stats.ErrorCount++
		return "", fmt.Errorf("spawn: deprecated / forbidden parameter 'out_fd'")
	}

	// Determine IO wiring based on optional stdin_fd/stdout_fd
	var (
		// Child side endpoints
		childStdin  io.Reader
		childStdout io.Writer
		childStderr io.Writer

		// Pipes we create (if any)
		prIn  *io.PipeReader
		pwIn  *io.PipeWriter
		prOut *os.File
		pwOut *os.File
		prErr *os.File
		pwErr *os.File

		// Exposed FDs only when we create them
		stdinFdCreated  = -1
		stdoutFdCreated = -1
		stderrFdCreated = -1
	)

	// stdout_fd handling (may be provided to stream to existing fd, e.g., 1)
	if v, ok := args["stdout_fd"]; ok {
		if f, ok2 := v.(float64); ok2 {
			target := int(f)
			switch target {
			case 1:
				if w, ok := e.fileDescriptors[1].(io.Writer); ok {
					childStdout = w
				} else {
					e.stats.ErrorCount++
					return "", fmt.Errorf("spawn: fd1 not writable")
				}
			case 2:
				if w, ok := e.fileDescriptors[2].(io.Writer); ok {
					childStdout = w
				} else {
					e.stats.ErrorCount++
					return "", fmt.Errorf("spawn: fd2 not writable")
				}
			default:
				if target >= 0 && target < len(e.fileDescriptors) {
					if w, ok := e.fileDescriptors[target].(io.Writer); ok {
						childStdout = w
					} else {
						e.stats.ErrorCount++
						return "", fmt.Errorf("spawn: stdout_fd %d is not writable", target)
					}
				} else {
					e.stats.ErrorCount++
					return "", fmt.Errorf("spawn: invalid stdout_fd %d", target)
				}
			}
		} else {
			e.stats.ErrorCount++
			return "", fmt.Errorf("spawn: stdout_fd must be a number")
		}
	}

	// stdin_fd handling (may be provided to feed existing fd to child stdin)
	if v, ok := args["stdin_fd"]; ok {
		if f, ok2 := v.(float64); ok2 {
			src := int(f)
			if src >= 0 && src < len(e.fileDescriptors) {
				if r, ok := e.fileDescriptors[src].(io.Reader); ok {
					childStdin = r
				} else {
					e.stats.ErrorCount++
					return "", fmt.Errorf("spawn: stdin_fd %d is not readable", src)
				}
			} else {
				e.stats.ErrorCount++
				return "", fmt.Errorf("spawn: invalid stdin_fd %d", src)
			}
		} else {
			e.stats.ErrorCount++
			return "", fmt.Errorf("spawn: stdin_fd must be a number")
		}
	}

	// If childStdin not provided, create a pipe and expose writable end to caller
	if childStdin == nil {
		prIn, pwIn = io.Pipe()
		childStdin = prIn
		stdinFdCreated = e.allocateFd()
	}

	// If childStdout not provided, create a pipe and expose readable end to caller
	if childStdout == nil {
		var err error
		prOut, pwOut, err = os.Pipe()
		if err != nil {
			e.stats.ErrorCount++
			if prIn != nil {
				_ = prIn.Close()
			}
			if pwIn != nil {
				_ = pwIn.Close()
			}
			return "", fmt.Errorf("spawn: failed to create stdout pipe: %w", err)
		}
		childStdout = pwOut
		stdoutFdCreated = e.allocateFd()
	}

	// Always capture stderr to a pipe for debugging context
	{
		var err error
		prErr, pwErr, err = os.Pipe()
		if err != nil {
			e.stats.ErrorCount++
			if prIn != nil {
				_ = prIn.Close()
			}
			if pwIn != nil {
				_ = pwIn.Close()
			}
			if prOut != nil {
				_ = prOut.Close()
			}
			if pwOut != nil {
				_ = pwOut.Close()
			}
			return "", fmt.Errorf("spawn: failed to create stderr pipe: %w", err)
		}
		childStderr = pwErr
		stderrFdCreated = e.allocateFd()
	}

	// Ensure fileDescriptors slice is large enough for any created fds
	maxCreated := -1
	for _, fd := range []int{stdinFdCreated, stdoutFdCreated, stderrFdCreated} {
		if fd > maxCreated {
			maxCreated = fd
		}
	}
	for len(e.fileDescriptors) <= maxCreated {
		e.fileDescriptors = append(e.fileDescriptors, nil)
	}
	if stdinFdCreated != -1 {
		e.fileDescriptors[stdinFdCreated] = pwIn
	}
	if stdoutFdCreated != -1 {
		e.fileDescriptors[stdoutFdCreated] = prOut
	}
	if stderrFdCreated != -1 {
		e.fileDescriptors[stderrFdCreated] = prErr
	}

	// Resolve llmsh executable path (C2 -> C1)
	exeName := "llmsh"
	if runtime.GOOS == "windows" {
		exeName = "llmsh.exe"
	}
	resolvedPath := ""
	// 1. sibling path relative to current executable directory
	if selfPath, err := os.Executable(); err == nil {
		baseDir := filepath.Dir(selfPath)
		cand := filepath.Join(baseDir, exeName)
		if fi, err2 := os.Stat(cand); err2 == nil && fi.Mode()&0111 != 0 {
			resolvedPath = cand
		}
	}
	// 1b. current working directory (useful in dev/test runs)
	if resolvedPath == "" {
		if cwd, err := os.Getwd(); err == nil {
			cand := filepath.Join(cwd, exeName)
			if fi, err2 := os.Stat(cand); err2 == nil && fi.Mode()&0111 != 0 {
				resolvedPath = cand
			}
		}
	}
	// 2. fallback to PATH
	if resolvedPath == "" {
		if lp, err := exec.LookPath(exeName); err == nil {
			resolvedPath = lp
		}
	}
	if resolvedPath == "" {
		e.stats.ErrorCount++
		// Close pipes to avoid leaks
		pwIn.Close()
		pwOut.Close()
		pwErr.Close()
		prIn.Close()
		prOut.Close()
		prErr.Close()
		return "", fmt.Errorf("spawn: process_spawn_error: cannot locate llmsh executable")
	}

	// Prepare command arguments (B1: single process via -c)
	argsList := []string{"-c", script}

	// VFS FD reuse (D): if parent provided attach FD(s), propagate via --vfs-fds to child llmsh
	if e.vfsMuxFDs != "" {
		argsList = append([]string{"--vfs-fds", e.vfsMuxFDs}, argsList...)
	}

	cmd := exec.Command(resolvedPath, argsList...)
	cmd.Stdin = childStdin
	cmd.Stdout = childStdout
	cmd.Stderr = childStderr

	// Environment: propagate existing only
	cmd.Env = os.Environ()

	// Launch process
	if err := cmd.Start(); err != nil {
		e.stats.ErrorCount++
		if pwIn != nil {
			_ = pwIn.Close()
		}
		if prIn != nil {
			_ = prIn.Close()
		}
		if pwOut != nil {
			_ = pwOut.Close()
		}
		if prOut != nil {
			_ = prOut.Close()
		}
		if pwErr != nil {
			_ = pwErr.Close()
		}
		if prErr != nil {
			_ = prErr.Close()
		}
		return "", fmt.Errorf("spawn: process_spawn_error: failed to start llmsh: %w", err)
	}

	// If we created stdout/stderr pipes, parent does not need the write ends; close them immediately
	if pwOut != nil {
		_ = pwOut.Close()
	}
	if pwErr != nil {
		_ = pwErr.Close()
	}

	pid := cmd.Process.Pid

	// Do not force-close child's stdin; --no-stdin is advisory for LLM planning only.

	// Track command info and completion (associate created fds)
	running := &RunningCommand{commandName: fmt.Sprintf("%s -c", exeName), cmd: cmd, done: make(chan struct{})}
	e.commandsMutex.Lock()
	if stdinFdCreated != -1 {
		e.runningCommands[stdinFdCreated] = running
	}
	if stdoutFdCreated != -1 {
		e.runningCommands[stdoutFdCreated] = running
	}
	if stderrFdCreated != -1 {
		e.runningCommands[stderrFdCreated] = running
	}
	e.commandsMutex.Unlock()

	// Also track globally (even when no fds were created)
	e.procsMutex.Lock()
	e.runningAll = append(e.runningAll, running)
	e.procsMutex.Unlock()

	// Goroutine to wait and close writers when process exits
	go func(rc *RunningCommand) {
		err := rc.cmd.Wait()
		// Write ends were already closed in parent; child close will drive EOF on read ends.
		// stdin reader close (child side) triggers here after process exit; ensure parent writer closed when done externally via close tool.
		if err != nil {
			// Write error to stderr pipe if still open
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				// Provide exit code message
				// Best-effort write; ignore if already closed
				// (skipped) pwErr is closed in parent; child already exited
			} else {
				// (skipped)
			}
		}
		// Signal completion
		close(rc.done)
	}(running)

	result := map[string]interface{}{
		"success":    true,
		"pid":        pid,
		"script_len": len(script),
	}
	if stdinFdCreated != -1 {
		result["stdin_fd"] = stdinFdCreated
	}
	if stdoutFdCreated != -1 {
		result["stdout_fd"] = stdoutFdCreated
	}
	if stderrFdCreated != -1 {
		result["stderr_fd"] = stderrFdCreated
	}
	return e.spawnSuccess(result)
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
// getSupportedCommands legacy helper (unused after refactor)
// func getSupportedCommands() []string { var cmds []string; for c := range builtin.Commands { cmds = append(cmds, c) }; sort.Strings(cmds); return cmds }

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

	// New semantics: on exit, close writer endpoints we own to propagate EOF, then wait for all spawned processes to finish
	e.closeWritableFds()
	// Signal EOF on fd0 via mux
	if e.stdioMux != nil {
		e.stdioMux.CloseStdinToEngine()
	}
	e.waitAllSpawned()

	// Return a special error to indicate exit request instead of calling os.Exit directly
	return fmt.Sprintf("Exit requested with code %d", code), fmt.Errorf("EXIT_REQUESTED:%d", code)
}

// closeWritableFds closes all writable file descriptors managed by the engine (fd>=3),
// excluding standard stdout/stderr, to ensure children receive EOF before waiting.
func (e *Engine) closeWritableFds() {
	e.chainMutex.RLock()
	fds := make([]interface{}, len(e.fileDescriptors))
	copy(fds, e.fileDescriptors)
	e.chainMutex.RUnlock()

	for fd, obj := range fds {
		if fd < 3 || obj == nil {
			continue // skip 0,1,2 and nils
		}
		if wc, ok := obj.(io.WriteCloser); ok {
			// Best-effort close; ignore errors here
			_ = wc.Close()
		}
	}
}

// waitAllSpawned waits for all tracked spawned processes to complete.
func (e *Engine) waitAllSpawned() {
	e.procsMutex.RLock()
	procs := make([]*RunningCommand, len(e.runningAll))
	copy(procs, e.runningAll)
	e.procsMutex.RUnlock()

	for _, rc := range procs {
		if rc == nil || rc.done == nil {
			continue
		}
		// Block until the process signals completion
		<-rc.done
	}
}

// executeOpen handles virtual file operations using the VFS
func (e *Engine) executeOpen(args map[string]interface{}) (string, error) {
	// Extract required path parameter
	pathVal, ok := args["path"]
	if !ok {
		e.stats.ErrorCount++
		return "", fmt.Errorf("missing required parameter: path")
	}
	path, ok := pathVal.(string)
	if !ok {
		e.stats.ErrorCount++
		return "", fmt.Errorf("path must be a string")
	}

	// Extract optional mode parameter (default: "r")
	mode := "r"
	if modeVal, ok := args["mode"]; ok {
		if m, ok := modeVal.(string); ok {
			mode = m
		}
	}

	// Validate mode and convert to os flags
	flag, perm, err := utils.ParseFileMode(mode)
	if err != nil {
		e.stats.ErrorCount++
		return "", err
	}

	// Use VFS to open the file
	if e.virtualFS == nil {
		e.stats.ErrorCount++
		return "", fmt.Errorf("virtual file system not available")
	}

	file, err := e.virtualFS.OpenFile(path, flag, perm)
	if err != nil {
		e.stats.ErrorCount++
		return "", fmt.Errorf("failed to open file '%s': %w", path, err)
	}

	// Assign a new file descriptor
	e.commandsMutex.Lock()
	fd := e.nextFd
	e.nextFd++

	// Extend fileDescriptors slice if needed
	for len(e.fileDescriptors) <= fd {
		e.fileDescriptors = append(e.fileDescriptors, nil)
	}
	e.fileDescriptors[fd] = file
	e.commandsMutex.Unlock()

	return fmt.Sprintf("Opened file '%s' with mode '%s', assigned fd=%d", path, mode, fd), nil
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

// executeHelp implements the help tool
func (e *Engine) executeHelp(args map[string]interface{}) (string, error) {
	keysInterface, ok := args["keys"].([]interface{})
	if !ok {
		e.stats.ErrorCount++
		return "", fmt.Errorf("help: missing or invalid 'keys' parameter")
	}

	keys := make([]string, len(keysInterface))
	for i, keyInterface := range keysInterface {
		key, ok := keyInterface.(string)
		if !ok {
			e.stats.ErrorCount++
			return "", fmt.Errorf("help: invalid key at index %d", i)
		}
		keys[i] = key
	}

	// Resolve llmsh executable (reuse logic similar to executeSpawn)
	exeName := "llmsh"
	if runtime.GOOS == "windows" {
		exeName = "llmsh.exe"
	}
	resolvedPath := ""
	if selfPath, err := os.Executable(); err == nil {
		baseDir := filepath.Dir(selfPath)
		cand := filepath.Join(baseDir, exeName)
		if fi, err2 := os.Stat(cand); err2 == nil && fi.Mode()&0111 != 0 {
			resolvedPath = cand
		}
	}
	if resolvedPath == "" {
		if cwd, err := os.Getwd(); err == nil {
			cand := filepath.Join(cwd, exeName)
			if fi, err2 := os.Stat(cand); err2 == nil && fi.Mode()&0111 != 0 {
				resolvedPath = cand
			}
		}
	}
	if resolvedPath == "" {
		if lp, err := exec.LookPath(exeName); err == nil {
			resolvedPath = lp
		}
	}
	if resolvedPath == "" {
		e.stats.ErrorCount++
		return "", fmt.Errorf("help: cannot locate llmsh executable")
	}

	// Build help script and execute
	script := "help"
	if len(keys) > 0 {
		script = script + " " + strings.Join(keys, " ")
	}
	cmd := exec.Command(resolvedPath, "-c", script)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		// Return stderr as part of error to aid debugging
		e.stats.ErrorCount++
		msg := strings.TrimSpace(stderrBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("help: %s", msg)
	}
	return stdoutBuf.String(), nil
}
