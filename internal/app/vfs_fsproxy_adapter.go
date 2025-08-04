package app

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/mako10k/llmcmd/internal/tools"
)

// VFSFSProxyAdapter provides FSProxy functionality through VFS interface
// This adapter allows llmsh to use FSProxy protocol transparently while maintaining
// compatibility with existing VirtualFileSystem interface.
type VFSFSProxyAdapter struct {
	mu           sync.RWMutex
	fsProxy      *FSProxyManager
	legacyVFS    tools.VirtualFileSystem // Fallback to legacy VFS if needed
	fdTable      *FileDescriptorTable
	clientID     string
	enableProxy  bool // Flag to enable/disable FSProxy usage
}

// NewVFSFSProxyAdapter creates a new VFS-FSProxy adapter
func NewVFSFSProxyAdapter(fsProxy *FSProxyManager, legacyVFS tools.VirtualFileSystem, enableProxy bool) *VFSFSProxyAdapter {
	adapter := &VFSFSProxyAdapter{
		fsProxy:     fsProxy,
		legacyVFS:   legacyVFS,
		enableProxy: enableProxy,
		clientID:    fmt.Sprintf("adapter-client-%d", generateUniqueID()),
	}
	
	// Use FSProxy's fd table if available, otherwise create our own
	if fsProxy != nil && fsProxy.fdTable != nil {
		adapter.fdTable = fsProxy.fdTable
	} else {
		adapter.fdTable = NewFileDescriptorTable()
	}
	
	return adapter
}

// OpenFile implements tools.VirtualFileSystem interface
// Opens a file using FSProxy if enabled, otherwise falls back to legacy VFS
func (adapter *VFSFSProxyAdapter) OpenFile(name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	
	// If FSProxy is enabled and available, use it
	if adapter.enableProxy && adapter.fsProxy != nil {
		return adapter.openFileThroughFSProxy(name, flag, perm)
	}
	
	// Fallback to legacy VFS
	if adapter.legacyVFS != nil {
		return adapter.legacyVFS.OpenFile(name, flag, perm)
	}
	
	// If no fallback available, return error
	return nil, fmt.Errorf("no file system available for opening file: %s", name)
}

// CreateTemp implements tools.VirtualFileSystem interface
// Creates a temporary file using FSProxy if enabled, otherwise falls back to legacy VFS
func (adapter *VFSFSProxyAdapter) CreateTemp(pattern string) (io.ReadWriteCloser, string, error) {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	
	// If FSProxy is enabled and available, use it
	if adapter.enableProxy && adapter.fsProxy != nil {
		return adapter.createTempThroughFSProxy(pattern)
	}
	
	// Fallback to legacy VFS
	if adapter.legacyVFS != nil {
		return adapter.legacyVFS.CreateTemp(pattern)
	}
	
	// If no fallback available, return error
	return nil, "", fmt.Errorf("no file system available for creating temp file with pattern: %s", pattern)
}

// RemoveFile implements tools.VirtualFileSystem interface
// Removes a file using FSProxy if enabled, otherwise falls back to legacy VFS
func (adapter *VFSFSProxyAdapter) RemoveFile(name string) error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	
	// If FSProxy is enabled and available, use it
	if adapter.enableProxy && adapter.fsProxy != nil {
		return adapter.removeFileThroughFSProxy(name)
	}
	
	// Fallback to legacy VFS
	if adapter.legacyVFS != nil {
		return adapter.legacyVFS.RemoveFile(name)
	}
	
	// If no fallback available, return error
	return fmt.Errorf("no file system available for removing file: %s", name)
}

// ListFiles implements tools.VirtualFileSystem interface
// Lists files using FSProxy if enabled, otherwise falls back to legacy VFS
func (adapter *VFSFSProxyAdapter) ListFiles() []string {
	adapter.mu.RLock()
	defer adapter.mu.RUnlock()
	
	// If FSProxy is enabled and available, use it
	if adapter.enableProxy && adapter.fsProxy != nil {
		return adapter.listFilesThroughFSProxy()
	}
	
	// Fallback to legacy VFS
	if adapter.legacyVFS != nil {
		return adapter.legacyVFS.ListFiles()
	}
	
	// If no fallback available, return empty list
	return []string{}
}

// FSProxy-specific implementations

// openFileThroughFSProxy opens a file using FSProxy protocol
func (adapter *VFSFSProxyAdapter) openFileThroughFSProxy(name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	// Convert os.O_* flags to FSProxy mode string
	mode := adapter.convertFlagToMode(flag)
	
	// Determine if this is a top-level operation (for now, assume true for adapter usage)
	isTopLevel := true
	
	// Use FSProxy's VFS to open the file
	if adapter.fsProxy.vfs == nil {
		return nil, fmt.Errorf("FSProxy VFS not available")
	}
	
	// Open file through FSProxy's VFS
	file, err := adapter.fsProxy.vfs.OpenFile(name, flag, perm)
	if err != nil {
		return nil, fmt.Errorf("failed to open file through FSProxy: %w", err)
	}
	
	// Store in fd table with adapter's client ID
	// Generate a unique fd for this file
	fd := adapter.generateFileDescriptor()
	adapter.fdTable.AddFile(fd, name, mode, adapter.clientID, isTopLevel, file)
	
	// Return a wrapped file handle that cleans up fd table on close
	return &FSProxyFileHandle{
		file:     file,
		fd:       fd,
		fdTable:  adapter.fdTable,
		filename: name,
	}, nil
}

// createTempThroughFSProxy creates a temporary file using FSProxy protocol
func (adapter *VFSFSProxyAdapter) createTempThroughFSProxy(pattern string) (io.ReadWriteCloser, string, error) {
	// Use FSProxy's VFS to create temp file
	if adapter.fsProxy.vfs == nil {
		return nil, "", fmt.Errorf("FSProxy VFS not available")
	}
	
	// CreateTemp through FSProxy's VFS (assuming it has CreateTemp method)
	file, filename, err := adapter.fsProxy.vfs.CreateTemp(pattern)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create temp file through FSProxy: %w", err)
	}
	
	// Store in fd table
	fd := adapter.generateFileDescriptor()
	adapter.fdTable.AddFile(fd, filename, "w+", adapter.clientID, true, file)
	
	// Return wrapped file handle
	return &FSProxyFileHandle{
		file:     file,
		fd:       fd,
		fdTable:  adapter.fdTable,
		filename: filename,
	}, filename, nil
}

// removeFileThroughFSProxy removes a file using FSProxy protocol
func (adapter *VFSFSProxyAdapter) removeFileThroughFSProxy(name string) error {
	// Use FSProxy's VFS to remove file
	if adapter.fsProxy.vfs == nil {
		return fmt.Errorf("FSProxy VFS not available")
	}
	
	// Remove file through FSProxy's VFS
	err := adapter.fsProxy.vfs.RemoveFile(name)
	if err != nil {
		return fmt.Errorf("failed to remove file through FSProxy: %w", err)
	}
	
	// Clean up any fd table entries for this file
	adapter.cleanupFileFromFDTable(name)
	
	return nil
}

// listFilesThroughFSProxy lists files using FSProxy protocol
func (adapter *VFSFSProxyAdapter) listFilesThroughFSProxy() []string {
	// Use FSProxy's VFS to list files
	if adapter.fsProxy.vfs == nil {
		return []string{}
	}
	
	// List files through FSProxy's VFS
	return adapter.fsProxy.vfs.ListFiles()
}

// Helper methods

// convertFlagToMode converts os.O_* flags to FSProxy mode string
func (adapter *VFSFSProxyAdapter) convertFlagToMode(flag int) string {
	switch {
	case flag&os.O_RDWR != 0:
		if flag&os.O_CREATE != 0 && flag&os.O_TRUNC != 0 {
			return "w+"
		} else if flag&os.O_CREATE != 0 && flag&os.O_APPEND != 0 {
			return "a+"
		}
		return "r+"
	case flag&os.O_WRONLY != 0:
		if flag&os.O_CREATE != 0 && flag&os.O_TRUNC != 0 {
			return "w"
		} else if flag&os.O_CREATE != 0 && flag&os.O_APPEND != 0 {
			return "a"
		}
		return "w"
	default:
		return "r"
	}
}

// generateFileDescriptor generates a unique file descriptor for this adapter
var fdCounter int = 2000 // Start from 2000 to avoid conflicts with FSProxy manager
var fdMutex sync.Mutex

func (adapter *VFSFSProxyAdapter) generateFileDescriptor() int {
	fdMutex.Lock()
	defer fdMutex.Unlock()
	fd := fdCounter
	fdCounter++
	return fd
}

// generateUniqueID generates a unique ID for client identification
var clientIDCounter int = 1
var clientIDMutex sync.Mutex

func generateUniqueID() int {
	clientIDMutex.Lock()
	defer clientIDMutex.Unlock()
	id := clientIDCounter
	clientIDCounter++
	return id
}

// cleanupFileFromFDTable removes file entries from fd table by filename
func (adapter *VFSFSProxyAdapter) cleanupFileFromFDTable(filename string) {
	// Get all files from fd table and remove matching filename entries
	allFiles := adapter.fdTable.GetAllFiles()
	for fd, openFile := range allFiles {
		if openFile.Filename == filename && openFile.ClientID == adapter.clientID {
			adapter.fdTable.RemoveFile(fd)
		}
	}
}

// FSProxyFileHandle wraps a file handle with fd table cleanup
type FSProxyFileHandle struct {
	file     io.ReadWriteCloser
	fd       int
	fdTable  *FileDescriptorTable
	filename string
	closed   bool
	mu       sync.Mutex
}

// Read implements io.Reader interface
func (fh *FSProxyFileHandle) Read(p []byte) (n int, err error) {
	return fh.file.Read(p)
}

// Write implements io.Writer interface
func (fh *FSProxyFileHandle) Write(p []byte) (n int, err error) {
	return fh.file.Write(p)
}

// Close implements io.Closer interface with fd table cleanup
func (fh *FSProxyFileHandle) Close() error {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	
	if fh.closed {
		return nil // Already closed
	}
	
	// Close the underlying file
	err := fh.file.Close()
	
	// Remove from fd table
	fh.fdTable.RemoveFile(fh.fd)
	
	// Mark as closed
	fh.closed = true
	
	return err
}
