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
		fdTable:   NewFileDescriptorTable(),
		clientID:  fmt.Sprintf("client-%d", time.Now().UnixNano()), // Generate unique client ID
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
