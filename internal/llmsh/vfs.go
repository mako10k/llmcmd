package llmsh

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/mako10k/llmcmd/internal/app"
	"github.com/mako10k/llmcmd/internal/tools"
)

// FileAccess represents file access permissions
type FileAccess int

const (
	AccessNone FileAccess = iota
	AccessRead
	AccessWrite
	AccessReadWrite
)

// VirtualFileSystem manages virtual files and pipes for llmsh

// LegacyVFSAdapter adapts llmsh VirtualFileSystem to tools.VirtualFileSystem interface
type LegacyVFSAdapter struct {
	vfs *VirtualFileSystem
}

// NewLegacyVFSAdapter creates an adapter for tools.VirtualFileSystem compatibility
func NewLegacyVFSAdapter(vfs *VirtualFileSystem) tools.VirtualFileSystem {
	return &LegacyVFSAdapter{vfs: vfs}
}

// OpenFile implements tools.VirtualFileSystem interface
func (adapter *LegacyVFSAdapter) OpenFile(name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	// Determine read/write mode from flags
	isRead := (flag&os.O_RDONLY) != 0 || (flag&os.O_RDWR) != 0
	isWrite := (flag&os.O_WRONLY) != 0 || (flag&os.O_RDWR) != 0
	isAppend := (flag & os.O_APPEND) != 0

	if isRead && isWrite {
		// For read-write mode, we need a different approach
		// For now, open for write and handle read separately
		writer, err := adapter.vfs.OpenForWrite(name, isAppend, true)
		if err != nil {
			return nil, err
		}
		// Return as ReadWriteCloser
		if rwc, ok := writer.(io.ReadWriteCloser); ok {
			return rwc, nil
		}
		return nil, fmt.Errorf("file %s does not support read-write mode", name)
	} else if isWrite {
		writer, err := adapter.vfs.OpenForWrite(name, isAppend, true)
		if err != nil {
			return nil, err
		}
		// Wrap WriteCloser as ReadWriteCloser
		return &writeOnlyWrapper{writer}, nil
	} else {
		// Read-only mode
		reader, err := adapter.vfs.OpenForRead(name, true)
		if err != nil {
			return nil, err
		}
		// Wrap ReadCloser as ReadWriteCloser
		return &readOnlyWrapper{reader}, nil
	}
}

// CreateTemp implements tools.VirtualFileSystem interface
func (adapter *LegacyVFSAdapter) CreateTemp(pattern string) (io.ReadWriteCloser, string, error) {
	// Generate a temporary file name
	tempName := fmt.Sprintf("temp_%s_%d", pattern, len(adapter.vfs.files))

	// Create virtual file
	vfile := NewVirtualFile(tempName)
	adapter.vfs.mu.Lock()
	adapter.vfs.files[tempName] = vfile
	adapter.vfs.mu.Unlock()

	return vfile, tempName, nil
}

// RemoveFile implements tools.VirtualFileSystem interface
func (adapter *LegacyVFSAdapter) RemoveFile(name string) error {
	adapter.vfs.mu.Lock()
	defer adapter.vfs.mu.Unlock()

	// Remove from virtual files
	if vfile, exists := adapter.vfs.files[name]; exists {
		vfile.Close()
		delete(adapter.vfs.files, name)
		return nil
	}

	// Remove from real files
	if file, exists := adapter.vfs.realFiles[name]; exists {
		file.Close()
		delete(adapter.vfs.realFiles, name)
		// Also try to remove the actual file
		os.Remove(name)
		return nil
	}

	return fmt.Errorf("file not found: %s", name)
}

// ListFiles implements tools.VirtualFileSystem interface
func (adapter *LegacyVFSAdapter) ListFiles() []string {
	return adapter.vfs.ListFiles()
}

// writeOnlyWrapper wraps WriteCloser to implement ReadWriteCloser
type writeOnlyWrapper struct {
	io.WriteCloser
}

func (w *writeOnlyWrapper) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("read not supported on write-only file")
}

// readOnlyWrapper wraps ReadCloser to implement ReadWriteCloser
type readOnlyWrapper struct {
	io.ReadCloser
}

func (r *readOnlyWrapper) Write(p []byte) (n int, err error) {
	return 0, fmt.Errorf("write not supported on read-only file")
}

// VirtualFileSystem manages virtual files and pipes for llmsh
type VirtualFileSystem struct {
	mu sync.RWMutex

	// Virtual files (temporary named pipes)
	files map[string]*VirtualFile

	// Real files (stdin, stdout, stderr, and dynamically opened files)
	realFiles map[string]io.ReadWriteCloser

	// File access permissions from -i/-o flags
	fileAccess map[string]FileAccess

	// FSProxy integration (Phase 3.1)
	fsProxyAdapter *app.VFSFSProxyAdapter
	enableFSProxy  bool
}

// VirtualFile represents a virtual file in memory with separate read/write positions
type VirtualFile struct {
	name   string
	buffer *bytes.Buffer
	closed bool
	mu     sync.RWMutex
}

// NewVirtualFile creates a new virtual file
func NewVirtualFile(name string) *VirtualFile {
	return &VirtualFile{
		name:   name,
		buffer: &bytes.Buffer{},
		closed: false,
	}
}

// Name returns the file name
func (vf *VirtualFile) Name() string {
	return vf.name
}

// Read reads from the virtual file - creates a new reader from current buffer content
func (vf *VirtualFile) Read(p []byte) (n int, err error) {
	vf.mu.RLock()
	defer vf.mu.RUnlock()

	if vf.closed {
		return 0, fmt.Errorf("file %s is closed", vf.name)
	}

	// Create a reader from current buffer content
	reader := bytes.NewReader(vf.buffer.Bytes())
	return reader.Read(p)
}

// Write writes to the virtual file
func (vf *VirtualFile) Write(p []byte) (n int, err error) {
	vf.mu.Lock()
	defer vf.mu.Unlock()

	if vf.closed {
		return 0, fmt.Errorf("file %s is closed", vf.name)
	}

	return vf.buffer.Write(p)
}

// GetReader returns a new reader for the entire content
func (vf *VirtualFile) GetReader() *bytes.Reader {
	vf.mu.RLock()
	defer vf.mu.RUnlock()
	return bytes.NewReader(vf.buffer.Bytes())
}

// Close closes the virtual file but keeps it available for reading
func (vf *VirtualFile) Close() error {
	vf.mu.Lock()
	defer vf.mu.Unlock()

	// Don't actually mark as closed for virtual files in VFS
	// This allows multiple open/close cycles
	// vf.closed = true
	return nil
}

// NewVirtualFileSystem creates a new VFS
func NewVirtualFileSystem(inputFiles, outputFiles []string) *VirtualFileSystem {
	vfs := &VirtualFileSystem{
		files:         make(map[string]*VirtualFile),
		realFiles:     make(map[string]io.ReadWriteCloser),
		fileAccess:    make(map[string]FileAccess),
		enableFSProxy: false, // Default to legacy mode
	}

	// Set up file access permissions
	// -i files are read-only
	for _, file := range inputFiles {
		if vfs.fileAccess[file] == AccessWrite {
			vfs.fileAccess[file] = AccessReadWrite // Already write, now both
		} else {
			vfs.fileAccess[file] = AccessRead
		}
	}

	// -o files are write-only
	for _, file := range outputFiles {
		if vfs.fileAccess[file] == AccessRead {
			vfs.fileAccess[file] = AccessReadWrite // Already read, now both
		} else {
			vfs.fileAccess[file] = AccessWrite
		}
	}

	// Set up standard streams
	vfs.realFiles["stdin"] = os.Stdin
	vfs.realFiles["stdout"] = os.Stdout
	vfs.realFiles["stderr"] = os.Stderr
	vfs.fileAccess["stdin"] = AccessRead
	vfs.fileAccess["stdout"] = AccessWrite
	vfs.fileAccess["stderr"] = AccessWrite

	return vfs
}

// NewVirtualFileSystemWithFSProxy creates a new VFS with optional FSProxy integration
func NewVirtualFileSystemWithFSProxy(inputFiles, outputFiles []string, enableFSProxy bool, fsProxyAdapter *app.VFSFSProxyAdapter) *VirtualFileSystem {
	vfs := NewVirtualFileSystem(inputFiles, outputFiles)

	// Configure FSProxy integration
	vfs.enableFSProxy = enableFSProxy
	vfs.fsProxyAdapter = fsProxyAdapter

	return vfs
}

// OpenForRead opens a file for reading
func (vfs *VirtualFileSystem) OpenForRead(filename string, isTopLevelCmd bool) (io.ReadCloser, error) {
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	// Check for already opened real files (including stdin)
	if realFile, exists := vfs.realFiles[filename]; exists {
		return realFile.(io.ReadCloser), nil
	}

	// Check for virtual files (no access restrictions)
	if vfile, exists := vfs.files[filename]; exists {
		return vfile, nil
	}

	// FSProxy Integration: Use FSProxy adapter if enabled and available
	if vfs.enableFSProxy && vfs.fsProxyAdapter != nil {
		context := "user"
		if !isTopLevelCmd {
			context = "internal"
		}

		readCloser, err := vfs.fsProxyAdapter.OpenForRead(filename, context)
		if err != nil {
			return nil, fmt.Errorf("FSProxy: cannot open file %s for reading: %v", filename, err)
		}
		return readCloser, nil
	}

	// Legacy mode: Direct file system access
	// If isTopLevelCmd=true, always try to open real file
	if isTopLevelCmd {
		file, err := os.Open(filename)
		if err != nil {
			return nil, fmt.Errorf("cannot open file %s: %v", filename, err)
		}
		// Cache the opened file
		vfs.realFiles[filename] = file
		return file, nil
	}

	// For isTopLevelCmd=false, check access permissions
	access, hasAccess := vfs.fileAccess[filename]
	if hasAccess && (access == AccessRead || access == AccessReadWrite) {
		// Try to open as real file
		file, err := os.Open(filename)
		if err != nil {
			return nil, fmt.Errorf("cannot open file %s: %v", filename, err)
		}

		// Cache the opened file
		vfs.realFiles[filename] = file
		return file, nil
	}

	// File not found or not accessible
	return nil, fmt.Errorf("file not found or not accessible for reading: %s", filename)
}

// OpenForWrite opens a file for writing
func (vfs *VirtualFileSystem) OpenForWrite(filename string, append bool, isTopLevelCmd bool) (io.WriteCloser, error) {
	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	// Check for already opened real files (including stdout, stderr)
	if realFile, exists := vfs.realFiles[filename]; exists {
		return realFile.(io.WriteCloser), nil
	}

	// Check for virtual files first (no access restrictions)
	if vfile, exists := vfs.files[filename]; exists {
		if !append {
			// Truncate if not appending
			vfile.buffer.Reset()
		}
		return vfile, nil
	}

	// FSProxy Integration: Use FSProxy adapter if enabled and available
	if vfs.enableFSProxy && vfs.fsProxyAdapter != nil {
		context := "user"
		if !isTopLevelCmd {
			context = "internal"
		}

		writeCloser, err := vfs.fsProxyAdapter.OpenForWrite(filename, append, context)
		if err != nil {
			return nil, fmt.Errorf("FSProxy: cannot open file %s for writing: %v", filename, err)
		}
		return writeCloser, nil
	}

	// Legacy mode: Direct file system access
	// If isTopLevelCmd=true, always try to open/create real file
	if isTopLevelCmd {
		var file *os.File
		var err error

		if append {
			file, err = os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		} else {
			file, err = os.Create(filename)
		}

		if err != nil {
			return nil, fmt.Errorf("cannot create file %s: %v", filename, err)
		}

		// Cache the opened file
		vfs.realFiles[filename] = file
		return file, nil
	}

	// For isTopLevelCmd=false, check access permissions
	access, hasAccess := vfs.fileAccess[filename]
	if hasAccess && (access == AccessWrite || access == AccessReadWrite) {
		// Try to open/create as real file
		var file *os.File
		var err error

		if append {
			file, err = os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		} else {
			file, err = os.Create(filename)
		}

		if err != nil {
			return nil, fmt.Errorf("cannot create file %s: %v", filename, err)
		}

		// Cache the opened file
		vfs.realFiles[filename] = file
		return file, nil
	}

	// If no real file access, create virtual file
	vfile := NewVirtualFile(filename)
	vfs.files[filename] = vfile
	return vfile, nil
}

// CreatePipe creates a virtual pipe between two commands
func (vfs *VirtualFileSystem) CreatePipe() (io.ReadCloser, io.WriteCloser, error) {
	pipeName := fmt.Sprintf("pipe_%d", len(vfs.files))
	vfile := NewVirtualFile(pipeName)

	vfs.mu.Lock()
	vfs.files[pipeName] = vfile
	vfs.mu.Unlock()

	// Return the same file for both read and write
	// VirtualFile implements both ReadCloser and WriteCloser
	return vfile, vfile, nil
}

// ListFiles returns a list of all virtual files
func (vfs *VirtualFileSystem) ListFiles() []string {
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	var files []string
	for name := range vfs.files {
		files = append(files, name)
	}

	return files
}

// CleanUp closes and removes all virtual files
func (vfs *VirtualFileSystem) CleanUp() error {
	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	for _, vfile := range vfs.files {
		vfile.Close()
	}

	vfs.files = make(map[string]*VirtualFile)

	// Close real files (except std streams)
	for name, file := range vfs.realFiles {
		if name != "stdin" && name != "stdout" && name != "stderr" {
			file.Close()
		}
	}

	return nil
}

// Implementation of tools.VirtualFileSystem interface for FSProxy integration

// OpenFile implements tools.VirtualFileSystem interface
func (vfs *VirtualFileSystem) OpenFile(name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	// If FSProxy is enabled, use adapter
	if vfs.enableFSProxy && vfs.fsProxyAdapter != nil {
		return vfs.fsProxyAdapter.OpenFile(name, flag, perm)
	}

	// Legacy implementation
	return vfs.openFileLegacy(name, flag, perm)
}

// CreateTemp implements tools.VirtualFileSystem interface
func (vfs *VirtualFileSystem) CreateTemp(pattern string) (io.ReadWriteCloser, string, error) {
	// If FSProxy is enabled, use adapter
	if vfs.enableFSProxy && vfs.fsProxyAdapter != nil {
		return vfs.fsProxyAdapter.CreateTemp(pattern)
	}

	// Legacy implementation
	return vfs.createTempLegacy(pattern)
}

// RemoveFile implements tools.VirtualFileSystem interface
func (vfs *VirtualFileSystem) RemoveFile(name string) error {
	// If FSProxy is enabled, use adapter
	if vfs.enableFSProxy && vfs.fsProxyAdapter != nil {
		return vfs.fsProxyAdapter.RemoveFile(name)
	}

	// Legacy implementation
	return vfs.removeFileLegacy(name)
}

// Legacy implementations for backwards compatibility

// openFileLegacy provides legacy file opening behavior
func (vfs *VirtualFileSystem) openFileLegacy(name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	// Check for already opened real files
	if realFile, exists := vfs.realFiles[name]; exists {
		return realFile, nil
	}

	// Check for virtual files
	if vfile, exists := vfs.files[name]; exists {
		return vfile, nil
	}

	// Try to open real file
	if flag&os.O_CREATE != 0 {
		// Create new file
		file, err := os.OpenFile(name, flag, perm)
		if err != nil {
			return nil, fmt.Errorf("cannot create file %s: %v", name, err)
		}
		vfs.realFiles[name] = file
		return file, nil
	} else {
		// Open existing file
		file, err := os.Open(name)
		if err != nil {
			// If file doesn't exist, create virtual file
			vfile := NewVirtualFile(name)
			vfs.files[name] = vfile
			return vfile, nil
		}
		vfs.realFiles[name] = file
		return file, nil
	}
}

// createTempLegacy provides legacy temporary file creation
func (vfs *VirtualFileSystem) createTempLegacy(pattern string) (io.ReadWriteCloser, string, error) {
	// Create a virtual temporary file
	tempName := fmt.Sprintf("%s_%d", pattern, len(vfs.files))
	vfile := NewVirtualFile(tempName)

	vfs.mu.Lock()
	vfs.files[tempName] = vfile
	vfs.mu.Unlock()

	return vfile, tempName, nil
}

// removeFileLegacy provides legacy file removal
func (vfs *VirtualFileSystem) removeFileLegacy(name string) error {
	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	// Remove from virtual files
	if vfile, exists := vfs.files[name]; exists {
		vfile.Close()
		delete(vfs.files, name)
		return nil
	}

	// Remove from real files
	if realFile, exists := vfs.realFiles[name]; exists {
		realFile.Close()
		delete(vfs.realFiles, name)
		// Also try to remove the actual file
		return os.Remove(name)
	}

	return fmt.Errorf("file not found: %s", name)
}
