package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// FileType represents the type of file in VFS
type FileType int

const (
	FileTypeRealFile FileType = iota // Real filesystem file
	FileTypeTempFile                 // Temporary file (O_TMPFILE)
	FileTypeVirtual                  // Virtual in-memory file (fallback)
)

// VFSEntry represents an entry in the virtual file system
type VFSEntry struct {
	Name     string             // File name or path
	FD       int                // File descriptor number
	Type     FileType           // File type (real file or pipe)
	File     io.ReadWriteCloser // Actual file handle
	Consumed bool               // Whether file has been consumed (for pipes)
}

// EnhancedVFS provides name <-> FD bidirectional mapping with file type awareness
type EnhancedVFS struct {
	nameToFD         map[string]int    // Name -> FD mapping
	fdToName         map[int]string    // FD -> Name mapping
	entries          map[int]*VFSEntry // FD -> Entry mapping
	nextFD           int               // Next available FD number (starting from 3)
	tempFiles        []int             // List of temporary file FDs for cleanup
	isTopLevel       bool              // Whether this VFS belongs to top-level llmcmd execution
	allowedRealFiles map[string]bool   // Real files allowed by top-level -i/-o
	mutex            sync.RWMutex
}

// NewEnhancedVFS creates a new enhanced VFS with standard FD initialization
func NewEnhancedVFS() *EnhancedVFS {
	return NewEnhancedVFSWithLevel(true) // Default to top-level
}

// NewEnhancedVFSWithLevel creates a new enhanced VFS with specified top-level status
func NewEnhancedVFSWithLevel(isTopLevel bool) *EnhancedVFS {
	vfs := &EnhancedVFS{
		nameToFD:         make(map[string]int),
		fdToName:         make(map[int]string),
		entries:          make(map[int]*VFSEntry),
		tempFiles:        make([]int, 0),
		isTopLevel:       isTopLevel,
		allowedRealFiles: make(map[string]bool),
		nextFD:           3, // Start from 3 (after stdin, stdout, stderr)
	}

	// Initialize standard file descriptors with real names from /proc/fd/
	vfs.initializeStandardFDs()

	return vfs
}

// initializeStandardFDs resolves real names for stdin, stdout, stderr
func (vfs *EnhancedVFS) initializeStandardFDs() {
	for fd := 0; fd <= 2; fd++ {
		realName := vfs.resolveStandardFD(fd)
		vfs.fdToName[fd] = realName
		vfs.nameToFD[realName] = fd

		// Create entry for standard FDs
		var file io.ReadWriteCloser
		switch fd {
		case 0:
			file = os.Stdin
		case 1:
			file = os.Stdout
		case 2:
			file = os.Stderr
		}

		vfs.entries[fd] = &VFSEntry{
			Name:     realName,
			FD:       fd,
			Type:     FileTypeRealFile, // Standard FDs are treated as real files
			File:     file,
			Consumed: false, // Standard FDs are not consumed initially
		}
	}
}

// resolveStandardFD resolves the real name of a standard file descriptor
func (vfs *EnhancedVFS) resolveStandardFD(fd int) string {
	procPath := fmt.Sprintf("/proc/self/fd/%d", fd)

	// Try to read the symlink target
	if target, err := os.Readlink(procPath); err == nil {
		// Clean the path and return
		return filepath.Clean(target)
	}

	// Fallback to proc path if readlink fails
	return procPath
}

// RegisterFile registers a file in VFS and returns assigned FD
func (vfs *EnhancedVFS) RegisterFile(name string, file io.ReadWriteCloser, fileType FileType) int {
	vfs.mutex.Lock()
	defer vfs.mutex.Unlock()

	// Check if name already exists
	if existingFD, exists := vfs.nameToFD[name]; exists {
		return existingFD
	}

	// Assign new FD
	fd := vfs.nextFD
	vfs.nextFD++

	// Create entry
	entry := &VFSEntry{
		Name: name,
		FD:   fd,
		Type: fileType,
		File: file,
	}

	// Register mappings
	vfs.nameToFD[name] = fd
	vfs.fdToName[fd] = name
	vfs.entries[fd] = entry

	return fd
}

// GetFileByName returns FD and entry for a given name
func (vfs *EnhancedVFS) GetFileByName(name string) (int, *VFSEntry, bool) {
	vfs.mutex.RLock()
	defer vfs.mutex.RUnlock()

	fd, exists := vfs.nameToFD[name]
	if !exists {
		return -1, nil, false
	}

	entry, exists := vfs.entries[fd]
	return fd, entry, exists
}

// GetFileByFD returns name and entry for a given FD
func (vfs *EnhancedVFS) GetFileByFD(fd int) (string, *VFSEntry, bool) {
	vfs.mutex.RLock()
	defer vfs.mutex.RUnlock()

	name, exists := vfs.fdToName[fd]
	if !exists {
		return "", nil, false
	}

	entry, exists := vfs.entries[fd]
	return name, entry, exists
}

// OpenFile opens or creates a file, registering it in VFS if needed
func (vfs *EnhancedVFS) OpenFile(name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	vfs.mutex.Lock()
	defer vfs.mutex.Unlock()

	// Check if file already exists in VFS
	if fd, exists := vfs.nameToFD[name]; exists {
		entry := vfs.entries[fd]

		// For temp files, check if already consumed
		if entry.Type == FileTypeTempFile && entry.Consumed && (flag&os.O_RDONLY != 0 || flag&os.O_RDWR != 0) {
			return nil, fmt.Errorf("temp file '%s' already consumed", name)
		}

		return entry.File, nil
	}

	// Determine file type and create appropriate file
	var file io.ReadWriteCloser
	var fileType FileType
	var err error

	// Check if it's a real filesystem path
	if filepath.IsAbs(name) || name[0] != '<' {
		// Real file system file
		file, err = os.OpenFile(name, flag, perm)
		if err != nil {
			return nil, err
		}
		// For top-level, default to real file; for internal, default to virtual
		if vfs.isTopLevel {
			fileType = FileTypeRealFile
		} else {
			fileType = FileTypeVirtual
		}
	} else {
		// Internal pipe (names starting with '<' are treated as pipes)
		file = &VirtualFile{
			name: name,
			data: []byte{},
			flag: flag,
			perm: perm,
		}
		fileType = FileTypeVirtual
	}

	// Register in VFS
	fd := vfs.nextFD
	vfs.nextFD++

	entry := &VFSEntry{
		Name:     name,
		FD:       fd,
		Type:     fileType,
		File:     file,
		Consumed: false,
	}

	vfs.nameToFD[name] = fd
	vfs.fdToName[fd] = name
	vfs.entries[fd] = entry

	return file, nil
}

// OpenFileWithContext opens a file with context awareness (required by tools.VirtualFileSystem interface)
func (vfs *EnhancedVFS) OpenFileWithContext(name string, flag int, perm os.FileMode, isInternal bool) (io.ReadWriteCloser, error) {
	if isInternal {
		// Internal access: use virtual files or temp files
		return vfs.OpenFile(name, flag, perm)
	} else {
		// External access: allow real files if configured
		if vfs.isTopLevel || vfs.IsRealFileAllowed(name) {
			return vfs.RegisterRealFile(name, flag, perm)
		}
		// Fallback to virtual file
		return vfs.OpenFile(name, flag, perm)
	}
}

// RegisterInputOutput registers input/output files based on execution level
// For top-level: -i, -o are real files (LLM hints + VFS read/write permission)
// For internal: -i, -o are temp files (internal context)
func (vfs *EnhancedVFS) RegisterInputOutput(name string, flag int, perm os.FileMode, isInput bool) (io.ReadWriteCloser, error) {
	if vfs.isTopLevel {
		// Top-level: treat as real file
		return vfs.RegisterRealFile(name, flag, perm)
	} else {
		// Internal level: create temp file with logical name
		if isInput {
			// For input, try to find existing file first
			if file, err := vfs.OpenFile(name, flag, perm); err == nil {
				return file, nil
			}
		}
		// Create temp file for internal I/O
		tempFile, err := vfs.CreateTempFile(name)
		if err != nil {
			return nil, err
		}
		return tempFile, nil
	}
}

// IsTopLevel returns whether this VFS is for top-level execution
func (vfs *EnhancedVFS) IsTopLevel() bool {
	vfs.mutex.RLock()
	defer vfs.mutex.RUnlock()
	return vfs.isTopLevel
}

// AllowRealFile adds a real file to the allowed list (for top-level -i/-o)
func (vfs *EnhancedVFS) AllowRealFile(name string) {
	vfs.mutex.Lock()
	defer vfs.mutex.Unlock()
	vfs.allowedRealFiles[name] = true
}

// IsRealFileAllowed checks if a real file is in the allowed list
func (vfs *EnhancedVFS) IsRealFileAllowed(name string) bool {
	vfs.mutex.RLock()
	defer vfs.mutex.RUnlock()
	return vfs.allowedRealFiles[name]
}

// RegisterRealFile registers a real filesystem file (for -i, -o command line options)
func (vfs *EnhancedVFS) RegisterRealFile(name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	vfs.mutex.Lock()
	defer vfs.mutex.Unlock()

	// For non-top-level, check if file is in allowed list
	if !vfs.isTopLevel && !vfs.allowedRealFiles[name] {
		return nil, fmt.Errorf("access denied: real file '%s' not allowed in internal context", name)
	}

	// Check if file already exists in VFS
	if fd, exists := vfs.nameToFD[name]; exists {
		entry := vfs.entries[fd]
		// Update type to real file if it was previously registered as virtual
		entry.Type = FileTypeRealFile
		return entry.File, nil
	}

	// Open real file
	file, err := os.OpenFile(name, flag, perm)
	if err != nil {
		return nil, fmt.Errorf("failed to open real file %s: %w", name, err)
	}

	// Register in VFS as real file
	fd := vfs.nextFD
	vfs.nextFD++

	entry := &VFSEntry{
		Name:     name,
		FD:       fd,
		Type:     FileTypeRealFile,
		File:     file,
		Consumed: false,
	}

	vfs.nameToFD[name] = fd
	vfs.fdToName[fd] = name
	vfs.entries[fd] = entry

	// Add to allowed list if top-level (for inheritance)
	if vfs.isTopLevel {
		vfs.allowedRealFiles[name] = true
	}

	return file, nil
}

// IsRealFile checks if a file is registered as a real file
func (vfs *EnhancedVFS) IsRealFile(name string) bool {
	vfs.mutex.RLock()
	defer vfs.mutex.RUnlock()

	if fd, exists := vfs.nameToFD[name]; exists {
		if entry, exists := vfs.entries[fd]; exists {
			return entry.Type == FileTypeRealFile
		}
	}
	return false
}

// GetRealFiles returns all real files registered in VFS
func (vfs *EnhancedVFS) GetRealFiles() []string {
	vfs.mutex.RLock()
	defer vfs.mutex.RUnlock()

	realFiles := make([]string, 0)
	for _, entry := range vfs.entries {
		if entry.Type == FileTypeRealFile && entry.FD > 2 { // Exclude stdin, stdout, stderr
			realFiles = append(realFiles, entry.Name)
		}
	}
	return realFiles
}

// InheritAllowedFiles inherits allowed real files from parent VFS (for llmsh integration)
func (vfs *EnhancedVFS) InheritAllowedFiles(parentVFS *EnhancedVFS) {
	vfs.mutex.Lock()
	defer vfs.mutex.Unlock()

	if parentVFS != nil {
		parentVFS.mutex.RLock()
		defer parentVFS.mutex.RUnlock()

		// Copy allowed real files from parent
		for filename := range parentVFS.allowedRealFiles {
			vfs.allowedRealFiles[filename] = true
		}
	}
}

// GetAllowedRealFiles returns list of allowed real files
func (vfs *EnhancedVFS) GetAllowedRealFiles() []string {
	vfs.mutex.RLock()
	defer vfs.mutex.RUnlock()

	files := make([]string, 0, len(vfs.allowedRealFiles))
	for filename := range vfs.allowedRealFiles {
		files = append(files, filename)
	}
	return files
}

// MarkConsumed marks a temp file as consumed
func (vfs *EnhancedVFS) MarkConsumed(name string) {
	vfs.mutex.Lock()
	defer vfs.mutex.Unlock()

	if fd, exists := vfs.nameToFD[name]; exists {
		if entry, exists := vfs.entries[fd]; exists && (entry.Type == FileTypeTempFile || entry.Type == FileTypeVirtual) {
			entry.Consumed = true
		}
	}
}

// ListEntries returns all VFS entries for debugging
func (vfs *EnhancedVFS) ListEntries() map[int]*VFSEntry {
	vfs.mutex.RLock()
	defer vfs.mutex.RUnlock()

	result := make(map[int]*VFSEntry)
	for fd, entry := range vfs.entries {
		result[fd] = entry
	}
	return result
}

// CreateTempFile creates a temporary file using O_TMPFILE for the given logical name
func (vfs *EnhancedVFS) CreateTempFile(logicalName string) (*os.File, error) {
	vfs.mutex.Lock()
	defer vfs.mutex.Unlock()

	// Create temporary file using O_TMPFILE (0x410000 on Linux)
	file, err := os.OpenFile("/tmp", 0x410000|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file for %s: %w", logicalName, err)
	}

	// Get file descriptor
	fd := int(file.Fd())

	// Track for cleanup
	vfs.tempFiles = append(vfs.tempFiles, fd)

	// Register in VFS with temp file type
	vfsFD := vfs.nextFD
	vfs.nextFD++

	entry := &VFSEntry{
		Name:     logicalName,
		FD:       vfsFD,
		Type:     FileTypeTempFile,
		File:     file,
		Consumed: false,
	}

	vfs.nameToFD[logicalName] = vfsFD
	vfs.fdToName[vfsFD] = fmt.Sprintf("/proc/self/fd/%d", fd) // Real FD path for reference
	vfs.entries[vfsFD] = entry

	return file, nil
}

// ResolvePath resolves a logical name to real filesystem path (for temp files, returns proc path)
func (vfs *EnhancedVFS) ResolvePath(logicalName string) (string, bool) {
	vfs.mutex.RLock()
	defer vfs.mutex.RUnlock()

	if fd, exists := vfs.nameToFD[logicalName]; exists {
		if entry, exists := vfs.entries[fd]; exists {
			if entry.Type == FileTypeTempFile {
				// For O_TMPFILE, return the /proc/self/fd/N path
				if realPath, exists := vfs.fdToName[fd]; exists {
					return realPath, true
				}
			}
			// For other types, return the logical name
			return logicalName, true
		}
	}
	return logicalName, false // Return original name if not found
}

// Cleanup closes all temporary files (kernel will clean up O_TMPFILE files)
func (vfs *EnhancedVFS) Cleanup() {
	vfs.mutex.Lock()
	defer vfs.mutex.Unlock()

	// Close all temporary files - kernel will automatically delete O_TMPFILE files
	for _, fd := range vfs.tempFiles {
		if entry, exists := vfs.entries[fd]; exists && entry.File != nil {
			entry.File.Close()
		}
	}
	vfs.tempFiles = nil
}

// CreateTemp creates a temporary file (implements VirtualFileSystem interface)
func (vfs *EnhancedVFS) CreateTemp(pattern string) (io.ReadWriteCloser, string, error) {
	file, err := vfs.CreateTempFile(pattern)
	if err != nil {
		return nil, "", err
	}

	// Return logical name for compatibility
	return file, pattern, nil
}

// RemoveFile removes a file (implements VirtualFileSystem interface)
func (vfs *EnhancedVFS) RemoveFile(name string) error {
	vfs.mutex.Lock()
	defer vfs.mutex.Unlock()

	if fd, exists := vfs.nameToFD[name]; exists {
		// Close file if open (for O_TMPFILE, this will automatically delete it)
		if entry, exists := vfs.entries[fd]; exists && entry.File != nil {
			entry.File.Close()
		}

		// Remove from mappings
		delete(vfs.nameToFD, name)
		delete(vfs.fdToName, fd)
		delete(vfs.entries, fd)

		// Remove from tempFiles slice
		for i, tempFD := range vfs.tempFiles {
			if tempFD == fd {
				vfs.tempFiles = append(vfs.tempFiles[:i], vfs.tempFiles[i+1:]...)
				break
			}
		}
	}

	return nil
}

// ListFiles lists all files (implements VirtualFileSystem interface)
func (vfs *EnhancedVFS) ListFiles() []string {
	vfs.mutex.RLock()
	defer vfs.mutex.RUnlock()

	files := make([]string, 0, len(vfs.entries))
	for _, entry := range vfs.entries {
		status := ""
		if entry.Consumed {
			status = " (consumed)"
		}
		files = append(files, entry.Name+status)
	}
	return files
}
