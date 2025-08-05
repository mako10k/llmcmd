package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mako10k/llmcmd/internal/tools"
)

// OpenFile represents an open file descriptor with metadata
type OpenFile struct {
	FileNo     int                `json:"fileno"`
	Filename   string             `json:"filename"`
	Mode       string             `json:"mode"`
	ClientID   string             `json:"client_id"`
	OpenedAt   time.Time          `json:"opened_at"`
	IsTopLevel bool               `json:"is_top_level"`
	Handle     io.ReadWriteCloser `json:"-"` // File handle (not serialized)
}

// FileDescriptorTable manages file descriptors with metadata
type FileDescriptorTable struct {
	mu    sync.RWMutex
	files map[int]*OpenFile // fileno -> OpenFile
}

// NewFileDescriptorTable creates a new file descriptor table
func NewFileDescriptorTable() *FileDescriptorTable {
	return &FileDescriptorTable{
		files: make(map[int]*OpenFile),
	}
}

// AddFile adds a new file to the table
func (fdt *FileDescriptorTable) AddFile(fileno int, filename, mode, clientID string, isTopLevel bool, handle io.ReadWriteCloser) {
	fdt.mu.Lock()
	defer fdt.mu.Unlock()

	fdt.files[fileno] = &OpenFile{
		FileNo:     fileno,
		Filename:   filename,
		Mode:       mode,
		ClientID:   clientID,
		OpenedAt:   time.Now(),
		IsTopLevel: isTopLevel,
		Handle:     handle,
	}
}

// GetFile retrieves a file by file descriptor
func (fdt *FileDescriptorTable) GetFile(fileno int) (*OpenFile, bool) {
	fdt.mu.RLock()
	defer fdt.mu.RUnlock()

	file, exists := fdt.files[fileno]
	return file, exists
}

// RemoveFile removes a file from the table
func (fdt *FileDescriptorTable) RemoveFile(fileno int) bool {
	fdt.mu.Lock()
	defer fdt.mu.Unlock()

	if _, exists := fdt.files[fileno]; exists {
		delete(fdt.files, fileno)
		return true
	}
	return false
}

// GetFilesByClient returns all files opened by a specific client
func (fdt *FileDescriptorTable) GetFilesByClient(clientID string) []int {
	fdt.mu.RLock()
	defer fdt.mu.RUnlock()

	var filenos []int
	for fileno, file := range fdt.files {
		if file.ClientID == clientID {
			filenos = append(filenos, fileno)
		}
	}
	return filenos
}

// GetAllFiles returns all open files
func (fdt *FileDescriptorTable) GetAllFiles() map[int]*OpenFile {
	fdt.mu.RLock()
	defer fdt.mu.RUnlock()

	result := make(map[int]*OpenFile)
	for fileno, file := range fdt.files {
		result[fileno] = file
	}
	return result
}

// BackgroundProcess represents a background process managed by FSProxy
type BackgroundProcess struct {
	mu        sync.RWMutex // Protects mutable fields
	PID       int          `json:"pid"`
	Command   string       `json:"command"`
	Args      []string     `json:"args"`
	Status    string       `json:"status"` // "running", "exited", "failed"
	StartTime time.Time    `json:"start_time"`
	EndTime   time.Time    `json:"end_time,omitempty"`
	Error     error        `json:"-"` // Error details (not serialized)
	Handle    *exec.Cmd    `json:"-"` // Process handle (not serialized)

	// I/O Stream management (Phase 1: Structure only)
	Stdin  io.WriteCloser `json:"-"` // Process stdin pipe (if created)
	Stdout io.ReadCloser  `json:"-"` // Process stdout pipe (if created)
	Stderr io.ReadCloser  `json:"-"` // Process stderr pipe (if created)
}

// GetStatus returns the current status in a thread-safe manner
func (bp *BackgroundProcess) GetStatus() string {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	return bp.Status
}

// SetStatus updates the status in a thread-safe manner
func (bp *BackgroundProcess) SetStatus(status string) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.Status = status
}

// ProcessTable manages background processes with thread-safe operations
type ProcessTable struct {
	mu        sync.RWMutex
	processes map[int]*BackgroundProcess
	nextPID   int
}

// NewProcessTable creates a new process table
func NewProcessTable() *ProcessTable {
	return &ProcessTable{
		processes: make(map[int]*BackgroundProcess),
		nextPID:   1000, // Start PIDs from 1000 to avoid conflicts
	}
}

// AddProcess adds a process to the table
func (pt *ProcessTable) AddProcess(process *BackgroundProcess) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.processes[process.PID] = process
}

// GetProcess retrieves a process by PID
func (pt *ProcessTable) GetProcess(pid int) *BackgroundProcess {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	return pt.processes[pid]
}

// RemoveProcess removes a process from the table
func (pt *ProcessTable) RemoveProcess(pid int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	delete(pt.processes, pid)
}

// ListProcesses returns all processes
func (pt *ProcessTable) ListProcesses() []*BackgroundProcess {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	processes := make([]*BackgroundProcess, 0, len(pt.processes))
	for _, process := range pt.processes {
		processes = append(processes, process)
	}
	return processes
}

// GeneratePID generates a new unique process ID
func (pt *ProcessTable) GeneratePID() int {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pid := pt.nextPID
	pt.nextPID++
	return pid
}

// FSProxyManager manages file system access for restricted child processes
type FSProxyManager struct {
	vfs       tools.VirtualFileSystem
	pipe      *os.File // Communication pipe with child process
	isVFSMode bool     // Whether to restrict file access to VFS only
	reader    *bufio.Reader
	writer    *bufio.Writer
	mutex     sync.Mutex // Protect concurrent access

	// File descriptor management
	nextFD    int                        // Next available file descriptor
	openFiles map[int]io.ReadWriteCloser // Map of fd to file handles (legacy)
	fdMutex   sync.RWMutex               // Protect fd operations

	// Enhanced fd management
	fdTable  *FileDescriptorTable // New fd management table
	clientID string               // Client identifier for this manager

	// Background process management
	processTable *ProcessTable // Process management table
}

// NewFSProxyManager creates a new FS proxy manager
func NewFSProxyManager(vfs tools.VirtualFileSystem, pipe *os.File, isVFSMode bool) *FSProxyManager {
	return &FSProxyManager{
		vfs:          vfs,
		pipe:         pipe,
		isVFSMode:    isVFSMode,
		reader:       bufio.NewReader(pipe),
		writer:       bufio.NewWriter(pipe),
		nextFD:       1000, // Start from 1000 to avoid conflicts
		openFiles:    make(map[int]io.ReadWriteCloser),
		fdTable:      NewFileDescriptorTable(),
		clientID:     fmt.Sprintf("client-%d", time.Now().UnixNano()), // Generate unique client ID
		processTable: NewProcessTable(),                               // Initialize process table
	}
}

// FSRequest represents a file system operation request
type FSRequest struct {
	Command  string // "OPEN", "READ", "WRITE", "CLOSE", "STREAM_READ", "STREAM_WRITE"
	Filename string
	Mode     string
	Context  string // "internal", "user" - access context
	Fileno   int
	Size     int
	Data     []byte

	// Stream management fields (Phase 2: Interface only)
	ProcessID  int    // For stream operations
	StreamType string // "stdin", "stdout", "stderr"
}

// FSResponse represents a file system operation response
type FSResponse struct {
	Status string // "OK", "ERROR"
	Data   string // Response data or error message
}

// HandleFSRequest handles file system requests from child processes
func (proxy *FSProxyManager) HandleFSRequest() error {
	// Ensure cleanup when function exits
	defer proxy.cleanup()

	for {
		request, err := proxy.readRequest()
		if err != nil {
			if err == io.EOF {
				// Child process closed the pipe - cleanup resources
				log.Printf("FS Proxy: Child process disconnected, cleaning up resources")
				return nil
			}
			log.Printf("FS Proxy: Error reading request: %v", err)
			continue
		}

		response := proxy.processRequest(request)

		if err := proxy.sendResponse(response); err != nil {
			log.Printf("FS Proxy: Error sending response: %v", err)
			return err
		}
	}
}

// readRequest reads and parses a request from the child process
func (proxy *FSProxyManager) readRequest() (FSRequest, error) {
	proxy.mutex.Lock()
	defer proxy.mutex.Unlock()

	line, err := proxy.reader.ReadString('\n')
	if err != nil {
		return FSRequest{}, err
	}

	line = strings.TrimSpace(line)

	// Try to parse as JSON first (for SPAWN and new commands)
	if strings.HasPrefix(line, "{") {
		return proxy.parseJSONRequest(line)
	}

	// Fallback to legacy line-based parsing
	return proxy.parseLegacyRequest(line)
}

func (proxy *FSProxyManager) parseJSONRequest(jsonStr string) (FSRequest, error) {
	var jsonReq map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &jsonReq); err != nil {
		return FSRequest{}, fmt.Errorf("invalid JSON: %w", err)
	}

	command, ok := jsonReq["command"].(string)
	if !ok {
		return FSRequest{}, fmt.Errorf("missing or invalid command field")
	}

	request := FSRequest{
		Command: command,
	}

	switch command {
	case "SPAWN":
		// Parse spawn parameters
		if args, ok := jsonReq["args"].([]interface{}); ok {
			stringArgs := make([]string, len(args))
			for i, arg := range args {
				if str, ok := arg.(string); ok {
					stringArgs[i] = str
				} else {
					return FSRequest{}, fmt.Errorf("spawn args must be strings")
				}
			}
			// Store args in Context field as JSON
			argsJSON, _ := json.Marshal(stringArgs)
			request.Context = string(argsJSON)
		} else {
			return FSRequest{}, fmt.Errorf("spawn requires args array")
		}

	case "STREAM_READ":
		if processID, ok := jsonReq["process_id"].(float64); ok {
			request.ProcessID = int(processID)
		} else {
			return FSRequest{}, fmt.Errorf("stream_read requires process_id")
		}
		if streamType, ok := jsonReq["stream_type"].(string); ok {
			if streamType != "stdout" && streamType != "stderr" {
				return FSRequest{}, fmt.Errorf("invalid stream_type: %s", streamType)
			}
			request.StreamType = streamType
		} else {
			return FSRequest{}, fmt.Errorf("stream_read requires stream_type")
		}
		if size, ok := jsonReq["size"].(float64); ok {
			request.Size = int(size)
		} else {
			return FSRequest{}, fmt.Errorf("stream_read requires size")
		}

	case "STREAM_WRITE":
		if processID, ok := jsonReq["process_id"].(float64); ok {
			request.ProcessID = int(processID)
		} else {
			return FSRequest{}, fmt.Errorf("stream_write requires process_id")
		}
		if streamType, ok := jsonReq["stream_type"].(string); ok {
			if streamType != "stdin" {
				return FSRequest{}, fmt.Errorf("invalid stream_type: %s", streamType)
			}
			request.StreamType = streamType
		} else {
			return FSRequest{}, fmt.Errorf("stream_write requires stream_type")
		}
		if data, ok := jsonReq["data"].(string); ok {
			request.Data = []byte(data)
			request.Size = len(request.Data)
		} else {
			return FSRequest{}, fmt.Errorf("stream_write requires data")
		}

	default:
		return FSRequest{}, fmt.Errorf("unknown JSON command: %s", command)
	}

	return request, nil
}

func (proxy *FSProxyManager) parseLegacyRequest(line string) (FSRequest, error) {
	parts := strings.Fields(line)

	if len(parts) == 0 {
		return FSRequest{}, fmt.Errorf("empty request")
	}

	request := FSRequest{
		Command: parts[0],
	}

	switch request.Command {
	case "OPEN":
		if len(parts) < 4 {
			return FSRequest{}, fmt.Errorf("OPEN requires filename, mode, and is_top_level")
		}
		request.Filename = parts[1]
		request.Mode = parts[2]

		// Parse is_top_level parameter
		isTopLevelStr := parts[3]
		if isTopLevelStr != "true" && isTopLevelStr != "false" {
			return FSRequest{}, fmt.Errorf("invalid is_top_level: %s", isTopLevelStr)
		}
		request.Context = isTopLevelStr // Store is_top_level flag in Context field

	case "READ":
		if len(parts) < 4 {
			return FSRequest{}, fmt.Errorf("READ requires fileno, size, and isTopLevel")
		}
		if fileno, err := strconv.Atoi(parts[1]); err != nil {
			return FSRequest{}, fmt.Errorf("invalid fileno: %s", parts[1])
		} else {
			request.Fileno = fileno
		}
		if size, err := strconv.Atoi(parts[2]); err != nil {
			return FSRequest{}, fmt.Errorf("invalid size: %s", parts[2])
		} else {
			request.Size = size
		}
		// Parse IsTopLevel parameter
		isTopLevel := parts[3]
		if isTopLevel != "true" && isTopLevel != "false" {
			return FSRequest{}, fmt.Errorf("invalid isTopLevel: %s", isTopLevel)
		}
		request.Context = isTopLevel // Store in Context field for now

	case "WRITE":
		if len(parts) < 3 {
			return FSRequest{}, fmt.Errorf("WRITE requires fileno and size")
		}
		if fileno, err := strconv.Atoi(parts[1]); err != nil {
			return FSRequest{}, fmt.Errorf("invalid fileno: %s", parts[1])
		} else {
			request.Fileno = fileno
		}
		if size, err := strconv.Atoi(parts[2]); err != nil {
			return FSRequest{}, fmt.Errorf("invalid size: %s", parts[2])
		} else {
			request.Size = size
		}

		// Read data of specified size
		if request.Size > 0 {
			data := make([]byte, request.Size)
			_, err := io.ReadFull(proxy.reader, data)
			if err != nil {
				return FSRequest{}, fmt.Errorf("failed to read data: %w", err)
			}
			request.Data = data
		}

	case "CLOSE":
		if len(parts) < 2 {
			return FSRequest{}, fmt.Errorf("CLOSE requires fileno")
		}
		if fileno, err := strconv.Atoi(parts[1]); err != nil {
			return FSRequest{}, fmt.Errorf("invalid fileno: %s", parts[1])
		} else {
			request.Fileno = fileno
		}

	case "STREAM_READ":
		// Phase 2: Basic command parsing for stream read
		if len(parts) < 3 {
			return FSRequest{}, fmt.Errorf("STREAM_READ requires process_id and stream_type")
		}
		if processID, err := strconv.Atoi(parts[1]); err != nil {
			return FSRequest{}, fmt.Errorf("invalid process_id: %s", parts[1])
		} else {
			request.ProcessID = processID
		}
		streamType := parts[2]
		if streamType != "stdout" && streamType != "stderr" {
			return FSRequest{}, fmt.Errorf("invalid stream_type: %s (must be stdout or stderr)", streamType)
		}
		request.StreamType = streamType

	case "STREAM_WRITE":
		// Phase 2: Basic command parsing for stream write
		if len(parts) < 4 {
			return FSRequest{}, fmt.Errorf("STREAM_WRITE requires process_id, stream_type, and size")
		}
		if processID, err := strconv.Atoi(parts[1]); err != nil {
			return FSRequest{}, fmt.Errorf("invalid process_id: %s", parts[1])
		} else {
			request.ProcessID = processID
		}
		streamType := parts[2]
		if streamType != "stdin" {
			return FSRequest{}, fmt.Errorf("invalid stream_type: %s (must be stdin)", streamType)
		}
		request.StreamType = streamType

		if size, err := strconv.Atoi(parts[3]); err != nil {
			return FSRequest{}, fmt.Errorf("invalid size: %s", parts[3])
		} else {
			request.Size = size
		}

		// Read data of specified size for stream write
		if request.Size > 0 {
			data := make([]byte, request.Size)
			_, err := io.ReadFull(proxy.reader, data)
			if err != nil {
				return FSRequest{}, fmt.Errorf("failed to read stream data: %w", err)
			}
			request.Data = data
		}

	case "LLM_CHAT":
		// LLM_CHAT is_top_level stdin_fd stdout_fd stderr_fd input_files_count prompt_length
		if len(parts) < 7 {
			return FSRequest{}, fmt.Errorf("LLM_CHAT requires is_top_level, stdin_fd, stdout_fd, stderr_fd, input_files_count, and prompt_length")
		}

		// Parse is_top_level
		isTopLevelStr := parts[1]
		if isTopLevelStr != "true" && isTopLevelStr != "false" {
			return FSRequest{}, fmt.Errorf("invalid is_top_level: %s", isTopLevelStr)
		}
		request.Context = isTopLevelStr

		// Parse file descriptors
		stdinFD, err := strconv.Atoi(parts[2])
		if err != nil || stdinFD < 0 {
			return FSRequest{}, fmt.Errorf("invalid stdin_fd: %s", parts[2])
		}
		stdoutFD, err := strconv.Atoi(parts[3])
		if err != nil || stdoutFD < 0 {
			return FSRequest{}, fmt.Errorf("invalid stdout_fd: %s", parts[3])
		}
		stderrFD, err := strconv.Atoi(parts[4])
		if err != nil || stderrFD < 0 {
			return FSRequest{}, fmt.Errorf("invalid stderr_fd: %s", parts[4])
		}

		// Parse data sizes
		inputFilesCount, err := strconv.Atoi(parts[5])
		if err != nil || inputFilesCount < 0 {
			return FSRequest{}, fmt.Errorf("invalid input_files_count: %s", parts[5])
		}
		promptLength, err := strconv.Atoi(parts[6])
		if err != nil || promptLength < 0 {
			return FSRequest{}, fmt.Errorf("invalid prompt_length: %s", parts[6])
		}

		// Store parameters in request fields
		request.ProcessID = stdinFD  // Reuse ProcessID for stdin_fd
		request.Fileno = stdoutFD    // Reuse Fileno for stdout_fd
		request.Size = stderrFD      // Reuse Size for stderr_fd

		// Read input files text
		var inputFilesText []byte
		if inputFilesCount > 0 {
			inputFilesText = make([]byte, inputFilesCount)
			_, err := io.ReadFull(proxy.reader, inputFilesText)
			if err != nil {
				return FSRequest{}, fmt.Errorf("failed to read input files data")
			}
		}

		// Read prompt text
		var promptText []byte
		if promptLength > 0 {
			promptText = make([]byte, promptLength)
			_, err := io.ReadFull(proxy.reader, promptText)
			if err != nil {
				return FSRequest{}, fmt.Errorf("failed to read prompt data")
			}
		}

		// Combine data: input_files_text + newline + prompt_text
		totalData := make([]byte, 0, inputFilesCount + 1 + promptLength)
		totalData = append(totalData, inputFilesText...)
		totalData = append(totalData, '\n')
		totalData = append(totalData, promptText...)
		request.Data = totalData

	case "LLM_QUOTA":
		// LLM_QUOTA has no parameters
		if len(parts) > 1 {
			return FSRequest{}, fmt.Errorf("LLM_QUOTA takes no parameters")
		}
		// Context will be used to check access control in handler

	default:
		return FSRequest{}, fmt.Errorf("unknown command: %s", request.Command)
	}

	return request, nil
}

// processRequest processes a file system request
func (proxy *FSProxyManager) processRequest(request FSRequest) FSResponse {
	switch request.Command {
	case "OPEN":
		return proxy.handleOpen(request.Filename, request.Mode, request.Context)
	case "READ":
		// Context field contains isTopLevel string ("true"/"false")
		isTopLevel := (request.Context == "true")
		return proxy.handleRead(request.Fileno, request.Size, isTopLevel)
	case "WRITE":
		return proxy.handleWrite(request.Fileno, request.Data)
	case "CLOSE":
		return proxy.handleClose(request.Fileno)
	case "SPAWN":
		// Parse args from Context field (JSON string)
		var args []string
		if err := json.Unmarshal([]byte(request.Context), &args); err != nil {
			return FSResponse{
				Status: "ERROR",
				Data:   fmt.Sprintf("invalid spawn args: %v", err),
			}
		}

		// Convert to map format expected by handleSpawn
		params := map[string]interface{}{
			"args": args,
		}

		result := proxy.handleSpawn(params)

		// Convert map result back to FSResponse
		if status, ok := result["status"].(string); ok {
			if data, ok := result["data"]; ok {
				return FSResponse{
					Status: status,
					Data:   fmt.Sprintf("%v", data),
				}
			}
		}

		return FSResponse{
			Status: "ERROR",
			Data:   "spawn handler returned invalid format",
		}

	case "STREAM_READ":
		// Phase 3: Delegate to stream read handler (actual implementation)
		return proxy.handleStreamRead(request.ProcessID, request.StreamType, request.Size)
	case "STREAM_WRITE":
		// Phase 3: Delegate to stream write handler (actual implementation)
		return proxy.handleStreamWrite(request.ProcessID, request.StreamType, request.Data)
	case "LLM_CHAT":
		// Handle LLM Chat request
		isTopLevel := (request.Context == "true")
		stdinFD := request.ProcessID  // Retrieved from ProcessID field
		stdoutFD := request.Fileno    // Retrieved from Fileno field
		stderrFD := request.Size      // Retrieved from Size field
		return proxy.handleLLMChat(isTopLevel, stdinFD, stdoutFD, stderrFD, request.Data)
	case "LLM_QUOTA":
		// Handle LLM Quota request
		return proxy.handleLLMQuota()
	default:
		return FSResponse{
			Status: "ERROR",
			Data:   fmt.Sprintf("unknown command: %s", request.Command),
		}
	}
}

// handleOpen handles OPEN requests according to FSProxy protocol
func (proxy *FSProxyManager) handleOpen(filename, mode, isTopLevelStr string) FSResponse {
	if proxy.vfs == nil {
		return FSResponse{
			Status: "ERROR",
			Data:   "VFS not available",
		}
	}

	// Validate filename
	if filename == "" {
		return FSResponse{
			Status: "ERROR",
			Data:   "failed to open file: filename cannot be empty",
		}
	}

	// Validate mode
	var flag int
	switch mode {
	case "r":
		flag = os.O_RDONLY
	case "w":
		flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	case "a":
		flag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	case "r+":
		flag = os.O_RDWR
	case "w+":
		flag = os.O_RDWR | os.O_CREATE | os.O_TRUNC
	case "a+":
		flag = os.O_RDWR | os.O_CREATE | os.O_APPEND
	default:
		return FSResponse{
			Status: "ERROR",
			Data:   fmt.Sprintf("invalid mode: %s", mode),
		}
	}

	// Parse is_top_level flag
	var isTopLevel bool
	switch isTopLevelStr {
	case "true":
		isTopLevel = true
	case "false":
		isTopLevel = false
	default:
		return FSResponse{
			Status: "ERROR",
			Data:   fmt.Sprintf("invalid is_top_level: %s", isTopLevelStr),
		}
	}

	// Open file through VFS
	file, err := proxy.vfs.OpenFile(filename, flag, 0644)
	if err != nil {
		return FSResponse{
			Status: "ERROR",
			Data:   fmt.Sprintf("failed to open file '%s': %v", filename, err),
		}
	}

	// VFS should return io.ReadWriteCloser compatible interface
	rwc := file

	// Assign file descriptor and store in both legacy and new table
	proxy.fdMutex.Lock()
	fd := proxy.nextFD
	proxy.nextFD++
	proxy.openFiles[fd] = rwc // Legacy table
	proxy.fdMutex.Unlock()

	// Store in new fd management table
	proxy.fdTable.AddFile(fd, filename, mode, proxy.clientID, isTopLevel, rwc)

	log.Printf("FS Proxy: Opened file '%s' with fd %d", filename, fd)

	return FSResponse{
		Status: "OK",
		Data:   fmt.Sprintf("%d", fd),
	}
}

// handleRead handles READ requests with isTopLevel support
func (proxy *FSProxyManager) handleRead(fileno int, size int, isTopLevel bool) FSResponse {
	// Get file from new fd management table
	openFile, exists := proxy.fdTable.GetFile(fileno)
	if !exists {
		return FSResponse{
			Status: "ERROR",
			Data:   fmt.Sprintf("invalid fileno: %d", fileno),
		}
	}

	// Log access type for debugging
	if isTopLevel {
		log.Printf("FS Proxy: READ with isTopLevel=true for fd %d (VFS server should access real file)", fileno)
	} else {
		log.Printf("FS Proxy: READ with isTopLevel=false for fd %d (VFS restricted environment)", fileno)
	}

	// Validate size parameter
	if size < 0 {
		return FSResponse{
			Status: "ERROR",
			Data:   "invalid size: negative value not allowed",
		}
	}

	// Handle zero size request
	if size == 0 {
		return FSResponse{
			Status: "OK",
			Data:   "0",
		}
	}

	// Read data from file
	buffer := make([]byte, size)
	n, err := openFile.Handle.Read(buffer)
	if err != nil && err != io.EOF {
		return FSResponse{
			Status: "ERROR",
			Data:   fmt.Sprintf("read error: %v", err),
		}
	}

	// Return actual data read (may be less than requested)
	actualData := buffer[:n]
	return FSResponse{
		Status: "OK",
		Data:   string(actualData), // Note: This is simplified - real implementation should handle binary data properly
	}
}

// handleWrite handles WRITE requests
func (proxy *FSProxyManager) handleWrite(fileno int, data []byte) FSResponse {
	// Get file from new fd management table
	openFile, exists := proxy.fdTable.GetFile(fileno)
	if !exists {
		return FSResponse{
			Status: "ERROR",
			Data:   fmt.Sprintf("invalid fileno: %d", fileno),
		}
	}

	// Write data to file
	n, err := openFile.Handle.Write(data)
	if err != nil {
		return FSResponse{
			Status: "ERROR",
			Data:   fmt.Sprintf("write error: %v", err),
		}
	}

	return FSResponse{
		Status: "OK",
		Data:   fmt.Sprintf("%d", n), // Return number of bytes written
	}
}

// handleClose handles CLOSE requests
func (proxy *FSProxyManager) handleClose(fileno int) FSResponse {
	// Get file from new fd management table
	openFile, exists := proxy.fdTable.GetFile(fileno)
	if !exists {
		return FSResponse{
			Status: "ERROR",
			Data:   fmt.Sprintf("invalid fileno: %d", fileno),
		}
	}

	// Close the file
	if err := openFile.Handle.Close(); err != nil {
		return FSResponse{
			Status: "ERROR",
			Data:   fmt.Sprintf("close error: %v", err),
		}
	}

	// Remove from both management tables
	proxy.fdMutex.Lock()
	delete(proxy.openFiles, fileno) // Legacy table
	proxy.fdMutex.Unlock()

	proxy.fdTable.RemoveFile(fileno) // New table

	log.Printf("FS Proxy: Closed file with fd %d", fileno)

	return FSResponse{
		Status: "OK",
		Data:   "",
	}
}

// sendResponse sends a response to the child process
func (proxy *FSProxyManager) sendResponse(response FSResponse) error {
	proxy.mutex.Lock()
	defer proxy.mutex.Unlock()

	// Send response line
	responseLine := fmt.Sprintf("%s %s\n", response.Status, response.Data)
	_, err := proxy.writer.WriteString(responseLine)
	if err != nil {
		return err
	}

	return proxy.writer.Flush()
}

// cleanup closes all open files when the proxy manager shuts down
func (proxy *FSProxyManager) cleanup() {
	// Get all open files from new fd table
	allFiles := proxy.fdTable.GetAllFiles()

	log.Printf("FS Proxy: Cleaning up %d open files for client %s", len(allFiles), proxy.clientID)

	// Close all files and remove from tables
	for fd, openFile := range allFiles {
		if openFile.Handle != nil {
			if err := openFile.Handle.Close(); err != nil {
				log.Printf("FS Proxy: Error closing fd %d (%s): %v", fd, openFile.Filename, err)
			} else {
				log.Printf("FS Proxy: Closed fd %d (%s)", fd, openFile.Filename)
			}
		}

		// Remove from new fd table
		proxy.fdTable.RemoveFile(fd)
	}

	// Clean up legacy table as well
	proxy.fdMutex.Lock()
	for fd, file := range proxy.openFiles {
		if file != nil {
			if err := file.Close(); err != nil {
				log.Printf("FS Proxy: Error closing legacy fd %d: %v", fd, err)
			}
		}
	}
	proxy.openFiles = make(map[int]io.ReadWriteCloser)
	proxy.fdMutex.Unlock()

	log.Printf("FS Proxy: Resource cleanup completed for client %s", proxy.clientID)
}

// handleSpawn handles spawn command requests
func (fm *FSProxyManager) handleSpawn(params map[string]interface{}) map[string]interface{} {
	// Validate input parameters
	command, ok := params["command"].(string)
	if !ok || command == "" {
		return map[string]interface{}{
			"status": "error",
			"error":  "command parameter is required and must be a non-empty string",
		}
	}

	args, ok := params["args"].([]string)
	if !ok {
		// Try to convert from []interface{} to []string
		if argsInterface, exists := params["args"].([]interface{}); exists {
			args = make([]string, len(argsInterface))
			for i, arg := range argsInterface {
				if argStr, ok := arg.(string); ok {
					args[i] = argStr
				} else {
					return map[string]interface{}{
						"status": "error",
						"error":  "all arguments must be strings",
					}
				}
			}
		} else {
			args = []string{} // Default to empty args
		}
	}

	// Generate unique process ID
	pid := fm.processTable.GeneratePID()

	// Create command
	cmd := exec.Command(command, args...)

	// Phase 1: Create I/O pipes for stream management (minimal implementation)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		// Fail-First: Return error immediately
		return map[string]interface{}{
			"status": "error",
			"error":  fmt.Sprintf("failed to create stdin pipe: %v", err),
		}
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		// Close stdin pipe and fail fast
		stdin.Close()
		return map[string]interface{}{
			"status": "error",
			"error":  fmt.Sprintf("failed to create stdout pipe: %v", err),
		}
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		// Close previous pipes and fail fast
		stdin.Close()
		stdout.Close()
		return map[string]interface{}{
			"status": "error",
			"error":  fmt.Sprintf("failed to create stderr pipe: %v", err),
		}
	}

	// Start the process
	err = cmd.Start()
	if err != nil {
		// Fail-First: Clean up pipes and return error immediately
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return map[string]interface{}{
			"status": "error",
			"error":  fmt.Sprintf("failed to start process: %v", err),
		}
	}

	// Create background process record with I/O streams
	process := &BackgroundProcess{
		PID:       pid,
		Command:   command,
		Args:      args,
		Status:    "running",
		StartTime: time.Now(),
		Handle:    cmd,
		Stdin:     stdin,
		Stdout:    stdout,
		Stderr:    stderr,
	}

	// Register process in table
	fm.processTable.AddProcess(process)

	// Start monitoring goroutine for process completion
	go fm.monitorProcess(process)

	return map[string]interface{}{
		"status":     "success",
		"process_id": pid,
	}
}

// monitorProcess monitors a background process and updates its status
func (fm *FSProxyManager) monitorProcess(process *BackgroundProcess) {
	// Wait for process completion
	err := process.Handle.Wait()

	// Clean up I/O streams (Phase 1: Basic cleanup)
	// Note: cmd.Wait() may already close pipes, so handle errors gracefully
	if process.Stdin != nil {
		if closeErr := process.Stdin.Close(); closeErr != nil {
			// Log only if it's not "already closed" error
			if !strings.Contains(closeErr.Error(), "file already closed") {
				log.Printf("Process %d: Failed to close stdin: %v", process.PID, closeErr)
			}
		}
	}
	if process.Stdout != nil {
		if closeErr := process.Stdout.Close(); closeErr != nil {
			// Log only if it's not "already closed" error
			if !strings.Contains(closeErr.Error(), "file already closed") {
				log.Printf("Process %d: Failed to close stdout: %v", process.PID, closeErr)
			}
		}
	}
	if process.Stderr != nil {
		if closeErr := process.Stderr.Close(); closeErr != nil {
			// Log only if it's not "already closed" error
			if !strings.Contains(closeErr.Error(), "file already closed") {
				log.Printf("Process %d: Failed to close stderr: %v", process.PID, closeErr)
			}
		}
	}

	// Update process status in a thread-safe manner
	process.EndTime = time.Now()
	if err != nil {
		process.SetStatus("failed")
		process.Error = err
	} else {
		process.SetStatus("exited")
	}

	log.Printf("Process %d (%s) completed with status: %s",
		process.PID, process.Command, process.GetStatus())
}

// handleStreamRead handles STREAM_READ requests (Phase 3: Actual implementation)
func (proxy *FSProxyManager) handleStreamRead(processID int, streamType string, size int) FSResponse {
	log.Printf("FS Proxy: STREAM_READ called - ProcessID: %d, StreamType: %s, Size: %d",
		processID, streamType, size)

	// Fail-First: Validate parameters
	if processID <= 0 {
		return FSResponse{
			Status: "ERROR",
			Data:   "invalid process_id: must be positive",
		}
	}

	if streamType != "stdout" && streamType != "stderr" {
		return FSResponse{
			Status: "ERROR",
			Data:   fmt.Sprintf("invalid stream_type: %s", streamType),
		}
	}

	if size < 0 {
		return FSResponse{
			Status: "ERROR",
			Data:   "invalid size: must be non-negative",
		}
	}

	// Handle zero size request
	if size == 0 {
		return FSResponse{
			Status: "OK",
			Data:   "",
		}
	}

	// Check if process exists
	process := proxy.processTable.GetProcess(processID)
	if process == nil {
		return FSResponse{
			Status: "ERROR",
			Data:   fmt.Sprintf("process not found: %d", processID),
		}
	}

	// Get the appropriate pipe based on stream type
	var pipe io.ReadCloser
	switch streamType {
	case "stdout":
		if process.Stdout == nil {
			return FSResponse{
				Status: "ERROR",
				Data:   "stdout pipe not available",
			}
		}
		pipe = process.Stdout
	case "stderr":
		if process.Stderr == nil {
			return FSResponse{
				Status: "ERROR",
				Data:   "stderr pipe not available",
			}
		}
		pipe = process.Stderr
	}

	// Limit buffer size to 16KB for safety and performance
	const maxBufferSize = 16 * 1024
	bufferSize := size
	if bufferSize > maxBufferSize {
		bufferSize = maxBufferSize
	}

	// Read data from the pipe
	buffer := make([]byte, bufferSize)
	n, err := pipe.Read(buffer)
	if err != nil {
		if err == io.EOF {
			// Process has finished, no more data
			return FSResponse{
				Status: "OK",
				Data:   "", // Empty data indicates EOF
			}
		}
		return FSResponse{
			Status: "ERROR",
			Data:   fmt.Sprintf("read error from %s: %v", streamType, err),
		}
	}

	// Return the actual data read
	actualData := buffer[:n]
	log.Printf("FS Proxy: Read %d bytes from %s of process %d", n, streamType, processID)

	return FSResponse{
		Status: "OK",
		Data:   string(actualData), // Note: This assumes text data - binary data needs different handling
	}
}

// handleStreamWrite handles STREAM_WRITE requests (Phase 3: Actual implementation)
func (proxy *FSProxyManager) handleStreamWrite(processID int, streamType string, data []byte) FSResponse {
	log.Printf("FS Proxy: STREAM_WRITE called - ProcessID: %d, StreamType: %s, DataSize: %d",
		processID, streamType, len(data))

	// Fail-First: Validate parameters
	if processID <= 0 {
		return FSResponse{
			Status: "ERROR",
			Data:   "invalid process_id: must be positive",
		}
	}

	if streamType != "stdin" {
		return FSResponse{
			Status: "ERROR",
			Data:   fmt.Sprintf("invalid stream_type: %s", streamType),
		}
	}

	// Check if process exists
	process := proxy.processTable.GetProcess(processID)
	if process == nil {
		return FSResponse{
			Status: "ERROR",
			Data:   fmt.Sprintf("process not found: %d", processID),
		}
	}

	// Check if stdin pipe is available
	if process.Stdin == nil {
		return FSResponse{
			Status: "ERROR",
			Data:   "stdin pipe not available",
		}
	}

	// Handle empty data case
	if len(data) == 0 {
		return FSResponse{
			Status: "OK",
			Data:   "0", // 0 bytes written
		}
	}

	// Write data to the stdin pipe
	n, err := process.Stdin.Write(data)
	if err != nil {
		// Handle broken pipe (process terminated)
		if strings.Contains(err.Error(), "broken pipe") {
			return FSResponse{
				Status: "ERROR",
				Data:   "process stdin closed (broken pipe)",
			}
		}
		return FSResponse{
			Status: "ERROR",
			Data:   fmt.Sprintf("write error to stdin: %v", err),
		}
	}

	log.Printf("FS Proxy: Wrote %d bytes to stdin of process %d", n, processID)

	return FSResponse{
		Status: "OK",
		Data:   fmt.Sprintf("%d", n), // Return number of bytes written
	}
}

// handleLLMChat handles LLM_CHAT requests according to FSProxy protocol
func (proxy *FSProxyManager) handleLLMChat(isTopLevel bool, stdinFD, stdoutFD, stderrFD int, data []byte) FSResponse {
	log.Printf("FS Proxy: LLM_CHAT called - TopLevel: %v, FDs: %d,%d,%d, DataSize: %d", 
		isTopLevel, stdinFD, stdoutFD, stderrFD, len(data))

	// Fail-First: Validate file descriptors
	if stdinFD < 0 || stdoutFD < 0 || stderrFD < 0 {
		return FSResponse{
			Status: "ERROR",
			Data:   "invalid file descriptors: must be non-negative",
		}
	}

	// Parse data to extract input files and prompt
	parts := strings.SplitN(string(data), "\n", 2)
	if len(parts) != 2 {
		return FSResponse{
			Status: "ERROR",
			Data:   "invalid data format: expected input_files_text\\nprompt_text",
		}
	}
	
	inputFilesText := parts[0]
	promptText := parts[1]

	// Prepare input files list
	var inputFiles []string
	if strings.TrimSpace(inputFilesText) != "" {
		inputFiles = strings.Split(strings.TrimSpace(inputFilesText), "\n")
	}

	log.Printf("FS Proxy: LLM_CHAT processing - InputFiles: %v, Prompt: %q", inputFiles, promptText)

	// Execute llmcmd as subprocess with VFS environment
	response, quotaStatus, err := proxy.executeLLMCmd(isTopLevel, inputFiles, promptText, stdinFD, stdoutFD, stderrFD)
	if err != nil {
		return FSResponse{
			Status: "ERROR",
			Data:   fmt.Sprintf("subprocess execution failed: %v", err),
		}
	}

	// Format response according to protocol: OK response_size quota_status
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return FSResponse{
			Status: "ERROR",
			Data:   fmt.Sprintf("failed to marshal response: %v", err),
		}
	}

	responseSize := len(responseJSON)
	
	// Protocol format: "OK response_size quota_status\n[response_json]"
	statusLine := fmt.Sprintf("%d %s", responseSize, quotaStatus)
	
	return FSResponse{
		Status: "OK",
		Data:   statusLine + "\n" + string(responseJSON),
	}
}

// executeLLMCmd executes llmcmd as subprocess with VFS environment injection
func (proxy *FSProxyManager) executeLLMCmd(isTopLevel bool, inputFiles []string, prompt string, stdinFD, stdoutFD, stderrFD int) (map[string]interface{}, string, error) {
	log.Printf("FS Proxy: executeLLMCmd - TopLevel: %v, InputFiles: %v, Prompt: %q", isTopLevel, inputFiles, prompt)

	// For MVP implementation, simulate subprocess execution without actual llmcmd call
	// TODO: Implement actual subprocess execution with proper pipe management
	
	// Simulate processing time
	time.Sleep(10 * time.Millisecond)

	// Create mock response based on input
	mockResponse := map[string]interface{}{
		"choices": []map[string]interface{}{
			{
				"message": map[string]interface{}{
					"content": fmt.Sprintf("Processed prompt: %s (simulated subprocess)", prompt),
				},
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     len(prompt) / 4, // Rough token estimate
			"completion_tokens": 25,
		},
	}

	quotaStatus := "175.0/5000 weighted tokens"

	log.Printf("FS Proxy: Mock subprocess execution completed")
	return mockResponse, quotaStatus, nil
}

// createPipeForFD creates a pipe for the specified file descriptor
func (proxy *FSProxyManager) createPipeForFD(fd int) (*os.File, error) {
	// For MVP implementation, handle standard FDs
	switch fd {
	case 0: // stdin
		// Create a pipe for stdin input
		r, w, err := os.Pipe()
		if err != nil {
			return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
		}
		// Return the read end for subprocess stdin
		w.Close() // Close write end for now (could be used for input later)
		return r, nil
	case 1: // stdout  
		// Create a pipe for stdout output
		r, w, err := os.Pipe()
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
		}
		// Return the write end for subprocess stdout
		r.Close() // Close read end for now (could be used to capture output later)
		return w, nil
	case 2: // stderr
		// Create a pipe for stderr output
		r, w, err := os.Pipe()
		if err != nil {
			return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
		}
		// Return the write end for subprocess stderr
		r.Close() // Close read end for now (could be used to capture output later)
		return w, nil
	default:
		// For other FDs, check if they exist in VFS
		if proxy.vfs != nil {
			// TODO: Map VFS file descriptors to OS pipes
			return nil, fmt.Errorf("VFS FD mapping for FD %d not yet implemented", fd)
		}
		return nil, fmt.Errorf("unsupported file descriptor: %d", fd)
	}
}

// handleLLMQuota handles LLM_QUOTA requests according to FSProxy protocol
func (proxy *FSProxyManager) handleLLMQuota() FSResponse {
	log.Printf("FS Proxy: LLM_QUOTA called")

	// Access Control: Check if this client is in an LLM execution context
	if !proxy.isLLMExecutionContext() {
		return FSResponse{
			Status: "ERROR",
			Data:   "LLM quota access denied",
		}
	}
	
	// Mock quota information for MVP
	// TODO: Implement actual quota tracking from shared quota manager
	quotaInfo := "150.0/5000 weighted tokens (3.0% used, 4850.0 remaining)"
	
	return FSResponse{
		Status: "OK",
		Data:   quotaInfo,
	}
}

// isLLMExecutionContext checks if the current client is in an LLM execution context
func (proxy *FSProxyManager) isLLMExecutionContext() bool {
	// For MVP implementation, we'll be permissive
	// TODO: Implement proper context tracking
	// - Track which clients were spawned by LLM_CHAT commands
	// - Maintain client context state in FSProxyManager
	// - Check client ID against LLM execution registry
	
	log.Printf("FS Proxy: LLM context check - allowing access for MVP (TODO: implement proper access control)")
	return true // Allow access for now
}
