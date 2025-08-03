package app

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/mako10k/llmcmd/internal/tools"
)

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
	openFiles map[int]io.ReadWriteCloser // Map of fd to file handles
	fdMutex   sync.RWMutex               // Protect fd operations
}

// NewFSProxyManager creates a new FS proxy manager
func NewFSProxyManager(vfs tools.VirtualFileSystem, pipe *os.File, isVFSMode bool) *FSProxyManager {
	return &FSProxyManager{
		vfs:       vfs,
		pipe:      pipe,
		isVFSMode: isVFSMode,
		reader:    bufio.NewReader(pipe),
		writer:    bufio.NewWriter(pipe),
		nextFD:    1000, // Start from 1000 to avoid conflicts
		openFiles: make(map[int]io.ReadWriteCloser),
	}
}

// FSRequest represents a file system operation request
type FSRequest struct {
	Command  string // "OPEN", "READ", "WRITE", "CLOSE"
	Filename string
	Mode     string
	Context  string // "internal", "user" - access context
	Fileno   int
	Size     int
	Data     []byte
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
	parts := strings.Fields(line)

	if len(parts) == 0 {
		return FSRequest{}, fmt.Errorf("empty request")
	}

	request := FSRequest{
		Command: parts[0],
	}

	switch request.Command {
	case "OPEN":
		if len(parts) < 3 {
			return FSRequest{}, fmt.Errorf("OPEN requires filename and mode")
		}
		request.Filename = parts[1]
		request.Mode = parts[2]

		// Optional context parameter (internal/user)
		if len(parts) >= 4 {
			context := parts[3]
			if context != "internal" && context != "user" {
				return FSRequest{}, fmt.Errorf("invalid context: %s", context)
			}
			request.Context = context
		} else {
			// Default to user context for compatibility
			request.Context = "user"
		}

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
	default:
		return FSResponse{
			Status: "ERROR",
			Data:   fmt.Sprintf("unknown command: %s", request.Command),
		}
	}
}

// handleOpen handles OPEN requests with context information
func (proxy *FSProxyManager) handleOpen(filename, mode, context string) FSResponse {
	if proxy.vfs == nil {
		return FSResponse{
			Status: "ERROR",
			Data:   "VFS not available",
		}
	}

	// Determine if this is internal access (LLM) or user access
	isInternal := (context == "internal")

	// Use context-aware VFS method if available
	if vfsWithContext, ok := proxy.vfs.(interface {
		OpenFileWithContext(string, string, bool) (interface{}, error)
	}); ok {
		file, err := vfsWithContext.OpenFileWithContext(filename, mode, isInternal)
		if err != nil {
			return FSResponse{
				Status: "ERROR",
				Data:   fmt.Sprintf("failed to open file '%s': %v", filename, err),
			}
		}

		// Convert to ReadWriteCloser
		var rwc io.ReadWriteCloser
		if f, ok := file.(io.ReadWriteCloser); ok {
			rwc = f
		} else {
			return FSResponse{
				Status: "ERROR",
				Data:   fmt.Sprintf("file does not implement ReadWriteCloser"),
			}
		}

		// Assign file descriptor and store
		proxy.fdMutex.Lock()
		fd := proxy.nextFD
		proxy.nextFD++
		proxy.openFiles[fd] = rwc
		proxy.fdMutex.Unlock()

		return FSResponse{
			Status: "OK",
			Data:   fmt.Sprintf("%d", fd),
		}
	}

	// Fall back to legacy VFS interface
	// Convert mode string to os flags
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

	// Open file through VFS
	file, err := proxy.vfs.OpenFile(filename, flag, 0644)
	if err != nil {
		return FSResponse{
			Status: "ERROR",
			Data:   fmt.Sprintf("failed to open file '%s': %v", filename, err),
		}
	}

	// Allocate new file descriptor and register the file
	proxy.fdMutex.Lock()
	fd := proxy.nextFD
	proxy.nextFD++
	proxy.openFiles[fd] = file
	proxy.fdMutex.Unlock()

	log.Printf("FS Proxy: Opened file '%s' with fd %d", filename, fd)

	return FSResponse{
		Status: "OK",
		Data:   strconv.Itoa(fd),
	}
}

// handleRead handles READ requests with isTopLevel support
func (proxy *FSProxyManager) handleRead(fileno int, size int, isTopLevel bool) FSResponse {
	proxy.fdMutex.RLock()
	file, exists := proxy.openFiles[fileno]
	proxy.fdMutex.RUnlock()

	if !exists {
		return FSResponse{
			Status: "ERROR",
			Data:   fmt.Sprintf("invalid fileno: %d", fileno),
		}
	}

	// If isTopLevel is true, the VFS server should open the real file (no restrictions)
	// For now, we'll log this behavior - the actual implementation would require
	// storing the isTopLevel context with each open file descriptor
	if isTopLevel {
		log.Printf("FS Proxy: READ with isTopLevel=true for fd %d (VFS server should access real file)", fileno)
	} else {
		log.Printf("FS Proxy: READ with isTopLevel=false for fd %d (VFS restricted environment)", fileno)
	}

	// Read data from file
	buffer := make([]byte, size)
	n, err := file.Read(buffer)
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
	// TODO: Implement proper fd to file mapping
	// For now, return placeholder
	return FSResponse{
		Status: "ERROR",
		Data:   "WRITE not yet implemented",
	}
}

// handleClose handles CLOSE requests
func (proxy *FSProxyManager) handleClose(fileno int) FSResponse {
	// TODO: Implement proper fd to file mapping
	// For now, return placeholder
	return FSResponse{
		Status: "ERROR",
		Data:   "CLOSE not yet implemented",
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
	proxy.fdMutex.Lock()
	defer proxy.fdMutex.Unlock()

	log.Printf("FS Proxy: Cleaning up %d open files", len(proxy.openFiles))

	for fd, file := range proxy.openFiles {
		if file != nil {
			if err := file.Close(); err != nil {
				log.Printf("FS Proxy: Error closing fd %d: %v", fd, err)
			}
		}
	}

	// Clear the map
	proxy.openFiles = make(map[int]io.ReadWriteCloser)
	log.Printf("FS Proxy: Resource cleanup completed")
}
