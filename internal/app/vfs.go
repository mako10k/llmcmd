package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// FileType represents the type of file in VFS
type FileType int

const (
	FileTypeRealFile FileType = iota
	FileTypeTempFile
	FileTypeVirtual
	FileTypeVirtualAnon
)

// File interface abstracts required file operations (subset of os.File)
// StdFile wraps *os.File for uniform naming
type StdFile struct {
	*os.File
	name string
}

func (s *StdFile) Name() string { return s.name }

// Stat and Sync forwarded when needed
func (s *StdFile) Stat() (os.FileInfo, error) { return s.File.Stat() }
func (s *StdFile) Sync() error                { return s.File.Sync() }

type virtualFileInfo struct {
	name string
	size int64
	mode os.FileMode
}

func (fi *virtualFileInfo) Name() string       { return fi.name }
func (fi *virtualFileInfo) Size() int64        { return fi.size }
func (fi *virtualFileInfo) Mode() os.FileMode  { return fi.mode }
func (fi *virtualFileInfo) ModTime() time.Time { return time.Now() }
func (fi *virtualFileInfo) IsDir() bool        { return fi.mode.IsDir() }
func (fi *virtualFileInfo) Sys() interface{}   { return nil }

// VirtualFile represents a virtual file in memory
// Implements File interface for os.File compatibility
type VirtualFile struct {
	name   string
	data   []byte
	offset int64
	flag   int
	perm   os.FileMode
	closed bool
}

// AccessModeFile wraps a File to enforce logical read / write permissions independent of underlying OS open flags.
type AccessModeFile struct {
	base       File
	allowRead  bool
	allowWrite bool
	name       string
}

func (a *AccessModeFile) Name() string { return a.name }

func (a *AccessModeFile) Read(p []byte) (int, error) {
	if !a.allowRead {
		return 0, fmt.Errorf("read not permitted on handle %s", a.name)
	}
	if r, ok := a.base.(interface{ Read([]byte) (int, error) }); ok {
		return r.Read(p)
	}
	return 0, fmt.Errorf("underlying file not readable: %s", a.name)
}

func (a *AccessModeFile) Write(p []byte) (int, error) {
	if !a.allowWrite {
		return 0, fmt.Errorf("write not permitted on handle %s", a.name)
	}
	if w, ok := a.base.(interface{ Write([]byte) (int, error) }); ok {
		return w.Write(p)
	}
	return 0, fmt.Errorf("underlying file not writable: %s", a.name)
}

func (a *AccessModeFile) Close() error {
	if c, ok := a.base.(interface{ Close() error }); ok {
		return c.Close()
	}
	return nil
}

func (a *AccessModeFile) Seek(offset int64, whence int) (int64, error) {
	if s, ok := a.base.(interface {
		Seek(int64, int) (int64, error)
	}); ok {
		return s.Seek(offset, whence)
	}
	return 0, fmt.Errorf("seek not supported: %s", a.name)
}

func (a *AccessModeFile) Sync() error {
	if s, ok := a.base.(interface{ Sync() error }); ok {
		return s.Sync()
	}
	return nil
}

func (a *AccessModeFile) Stat() (os.FileInfo, error) {
	if s, ok := a.base.(interface{ Stat() (os.FileInfo, error) }); ok {
		return s.Stat()
	}
	return nil, fmt.Errorf("stat not supported: %s", a.name)
}

// Name returns the name of the file (implements File interface)
func (f *VirtualFile) Name() string {
	return f.name
}

// Stat returns a FileInfo describing the file (implements File interface)
func (f *VirtualFile) Stat() (os.FileInfo, error) {
	// Return a simple FileInfo implementation
	return &virtualFileInfo{
		name: f.name,
		size: int64(len(f.data)),
		mode: f.perm,
	}, nil
}

// Sync commits the current contents to stable storage (implements File interface)
func (f *VirtualFile) Sync() error {
	// Virtual files don't need syncing
	return nil
}

// Truncate changes the size of the file (implements File interface)
func (f *VirtualFile) Truncate(size int64) error {
	if f.closed {
		return os.ErrClosed
	}
	if size < 0 {
		return fmt.Errorf("negative size")
	}
	if size == 0 {
		f.data = []byte{}
	} else if int64(len(f.data)) > size {
		f.data = f.data[:size]
	} else {
		// Extend with zeros
		newData := make([]byte, size)
		copy(newData, f.data)
		f.data = newData
	}
	if f.offset > size {
		f.offset = size
	}
	return nil
}

// Seek sets the offset for the next Read or Write (implements File interface)
func (f *VirtualFile) Seek(offset int64, whence int) (int64, error) {
	if f.closed {
		return 0, os.ErrClosed
	}
	switch whence {
	case 0: // SEEK_SET
		f.offset = offset
	case 1: // SEEK_CUR
		f.offset += offset
	case 2: // SEEK_END
		f.offset = int64(len(f.data)) + offset
	default:
		return 0, fmt.Errorf("invalid whence")
	}
	if f.offset < 0 {
		f.offset = 0
	}
	return f.offset, nil
}

// Read implements io.Reader with PIPE-like behavior (consume data)
func (f *VirtualFile) Read(p []byte) (n int, err error) {
	if f.closed {
		return 0, os.ErrClosed
	}
	if f.offset >= int64(len(f.data)) {
		return 0, io.EOF
	}
	n = copy(p, f.data[f.offset:])
	f.offset += int64(n)

	// PIPE behavior: once data is read, it's consumed and removed
	// This simulates pipe consumption where data can only be read once
	if f.offset >= int64(len(f.data)) {
		// All data has been read, mark as consumed
		f.data = nil // Clear data to prevent re-reading
	}

	return n, nil
}

// Write implements io.Writer
func (f *VirtualFile) Write(p []byte) (n int, err error) {
	if f.closed {
		return 0, os.ErrClosed
	}
	if f.flag&os.O_APPEND != 0 {
		f.data = append(f.data, p...)
	} else {
		// Extend data if necessary
		needed := f.offset + int64(len(p))
		if int64(len(f.data)) < needed {
			newData := make([]byte, needed)
			copy(newData, f.data)
			f.data = newData
		}
		copy(f.data[f.offset:], p)
		f.offset += int64(len(p))
	}
	return len(p), nil
}

// Close implements io.Closer
func (f *VirtualFile) Close() error {
	f.closed = true
	return nil
}

// Clone creates a copy of the VirtualFile with new flag and perm settings
// This allows the same virtual file data to be opened with different access modes
func (f *VirtualFile) Clone(flag int, perm os.FileMode) *VirtualFile {
	// Create a copy of the data
	dataCopy := make([]byte, len(f.data))
	copy(dataCopy, f.data)

	return &VirtualFile{
		name:   f.name,
		data:   dataCopy,
		offset: 0, // Reset offset for new handle
		flag:   flag,
		perm:   perm,
		closed: false,
	}
}

// VFSEntry represents an entry in the virtual file system
type VFSEntry struct {
	Name     string             // File name or path
	FD       int                // File descriptor number
	Type     FileType           // File type (real file or pipe)
	File     io.ReadWriteCloser // Actual file handle
	Consumed bool               // Whether file has been consumed (for pipes)
}

// VirtualFS provides name <-> FD bidirectional mapping with file type awareness
// This serves as the VFS Server in the 4-layer architecture
type VirtualFS struct {
	nameToFD         map[string]int    // Name -> FD mapping
	fdToName         map[int]string    // FD -> Name mapping
	entries          map[int]*VFSEntry // FD -> Entry mapping
	nextFD           int               // Next available FD number (starting from 3)
	tempFiles        []int             // List of temporary file FDs for cleanup
	isTopLevel       bool              // Whether this VFS belongs to top-level llmcmd execution
	allowedRealFiles map[string]bool   // Real files allowed by top-level -i/-o
	virtualMode      bool              // Virtual mode flag: only injected (-i/-o) real files permitted as real
	injectedReal     map[string]bool   // Set of injected real file names (for virtual mode gating)
	mutex            sync.RWMutex
}

// NewVFS creates a new enhanced VFS with standard FD initialization

// NewVFS creates a new VFS (top-level, non-virtual) - legacy helper
func NewVFS() *VirtualFS { return VFSWithOptions(true, false, nil) }

// VFSWithLevel legacy wrapper (kept for transitional compatibility)
func VFSWithLevel(isTopLevel bool) *VirtualFS { return VFSWithOptions(isTopLevel, false, nil) }

// VFSWithOptions creates a VFS with top-level flag, virtual mode, and injected real files
func VFSWithOptions(isTopLevel bool, virtualMode bool, injected []string) *VirtualFS {
	vfs := &VirtualFS{
		nameToFD:         make(map[string]int),
		fdToName:         make(map[int]string),
		entries:          make(map[int]*VFSEntry),
		tempFiles:        make([]int, 0),
		isTopLevel:       isTopLevel,
		allowedRealFiles: make(map[string]bool),
		virtualMode:      virtualMode,
		injectedReal:     make(map[string]bool),
		nextFD:           3, // Start from 3 (after stdin, stdout, stderr)
	}

	for _, f := range injected {
		if f == "" {
			continue
		}
		clean := filepath.Clean(f)
		abs := clean
		if a, err := filepath.Abs(clean); err == nil {
			abs = a
		}
		vfs.allowedRealFiles[clean] = true
		vfs.allowedRealFiles[abs] = true
		vfs.injectedReal[clean] = true
		vfs.injectedReal[abs] = true
	}

	// Initialize standard file descriptors with real names from /proc/fd/
	vfs.initializeStandardFDs()

	return vfs
}

// initializeStandardFDs resolves real names for stdin, stdout, stderr
func (vfs *VirtualFS) initializeStandardFDs() {
	for fd := 0; fd <= 2; fd++ {
		realName := vfs.resolveStandardFD(fd)
		vfs.fdToName[fd] = realName
		vfs.nameToFD[realName] = fd

		// Create entry for standard FDs
		var file File
		switch fd {
		case 0:
			file = &StdFile{File: os.Stdin, name: realName}
		case 1:
			file = &StdFile{File: os.Stdout, name: realName}
		case 2:
			file = &StdFile{File: os.Stderr, name: realName}
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
func (vfs *VirtualFS) resolveStandardFD(fd int) string {
	// Use standard names for standard file descriptors
	switch fd {
	case 0:
		return "stdin"
	case 1:
		return "stdout"
	case 2:
		return "stderr"
	default:
		// Only for non-standard FDs, try to resolve using /proc
		procPath := fmt.Sprintf("/proc/self/fd/%d", fd)

		// Try to read the symlink target
		if target, err := os.Readlink(procPath); err == nil {
			// Clean the path and return
			return filepath.Clean(target)
		}

		// Fallback to fd name if readlink fails
		return fmt.Sprintf("fd%d", fd)
	}
}

// RegisterFile registers a file in VFS and returns assigned FD
func (vfs *VirtualFS) RegisterFile(name string, rawFile io.ReadWriteCloser, fileType FileType) int {
	vfs.mutex.Lock()
	defer vfs.mutex.Unlock()

	// Check if name already exists
	if existingFD, exists := vfs.nameToFD[name]; exists {
		return existingFD
	}

	// Convert io.ReadWriteCloser to File interface
	var file io.ReadWriteCloser = rawFile

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
func (vfs *VirtualFS) GetFileByName(name string) (int, *VFSEntry, bool) {
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
func (vfs *VirtualFS) GetFileByFD(fd int) (string, *VFSEntry, bool) {
	vfs.mutex.RLock()
	defer vfs.mutex.RUnlock()

	name, exists := vfs.fdToName[fd]
	if !exists {
		return "", nil, false
	}

	entry, exists := vfs.entries[fd]
	return name, entry, exists
}

// OpenFile opens or creates a file, registering it in VFS if needed (implements VirtualFileSystem interface)
func (vfs *VirtualFS) OpenFile(name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	file, err := vfs.openFile(name, flag, perm)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// openFile is the internal implementation that returns File interface
func (vfs *VirtualFS) openFile(name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	vfs.mutex.Lock()
	defer vfs.mutex.Unlock()

	// Check if file already exists in VFS
	if fd, exists := vfs.nameToFD[name]; exists {
		entry := vfs.entries[fd]

		// Behavior varies by type
		switch entry.Type {
		case FileTypeTempFile:
			if entry.Consumed && (flag&os.O_RDONLY != 0 || flag&os.O_RDWR != 0) {
				return nil, fmt.Errorf("temp file '%s' already consumed", name)
			}
			return entry.File, nil
		case FileTypeRealFile:
			// Reopen underlying real file for a fresh handle
			rawFile, err := os.OpenFile(name, flag, perm)
			if err != nil {
				return nil, err
			}
			entry.File = rawFile
			vfs.entries[fd] = entry
			return rawFile, nil
		case FileTypeVirtual:
			if vf, ok := entry.File.(*VirtualFile); ok {
				clonedFile := vf.Clone(flag, perm)
				return clonedFile, nil
			}
			return entry.File, nil
		case FileTypeVirtualAnon:
			// Duplicate base FD (anonymous O_TMPFILE)
			if osf, ok := entry.File.(*os.File); ok {
				dupFD, err := syscall.Dup(int(osf.Fd()))
				if err != nil {
					return nil, fmt.Errorf("dup virtual anon fd: %w", err)
				}
				return os.NewFile(uintptr(dupFD), name), nil
			}
			return nil, fmt.Errorf("invalid anon virtual file entry type for %s", name)
		default:
			return entry.File, nil
		}
	}

	var file io.ReadWriteCloser
	var fileType FileType

	// Identify "real path" semantics (absolute path OR not an internal pipe marker '<')
	isRealPath := name != "" && (filepath.IsAbs(name) || name[0] != '<')

	if isRealPath {
		// Determine if this path is allowed to be real
		allowReal := (vfs.allowedRealFiles[name] && vfs.injectedReal[name]) || (!vfs.virtualMode && vfs.isTopLevel) || (vfs.allowedRealFiles[name] && !vfs.virtualMode)
		if allowReal {
			rawFile, err := os.OpenFile(name, flag, perm)
			if err != nil {
				return nil, err
			}
			file = rawFile
			fileType = FileTypeRealFile
		} else {
			// Virtualize non-injected real path using anonymous O_TMPFILE (fail-fast if unsupported)
			base, err := os.OpenFile("/tmp", 0x410000|os.O_RDWR, 0600) // O_TMPFILE|RDWR
			if err != nil {
				return nil, fmt.Errorf("virtual anon create failed for %s: %w", name, err)
			}
			file = base
			fileType = FileTypeVirtualAnon
		}
	} else {
		// Pipe / logical virtual file
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
// OpenFileSession replaces prior context-based open; now decision relies solely on VFS internal policy.
// Behavior:
//   - Top-level: real file (RegisterRealFile) unless policy denies (size/binary checks would be elsewhere)
//   - Non top-level: only allowedRealFiles become real; others -> error (Fail-First) or virtual depending on prior logic.
//     Current policy (post-hardening): deny + error for unallowed real paths (enforced in openFile).
func (vfs *VirtualFS) OpenFileSession(name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	// Special handling BEFORE normalization: standard streams
	if name == "stdin" || name == "stdout" || name == "stderr" {
		vfs.mutex.RLock()
		defer vfs.mutex.RUnlock()
		if fd, ok := vfs.nameToFD[name]; ok && fd <= 2 {
			if entry, exists := vfs.entries[fd]; exists {
				if f, ok2 := entry.File.(io.ReadWriteCloser); ok2 {
					return f, nil
				}
				if osf, isOS := entry.File.(*os.File); isOS {
					return osf, nil
				}
			}
		}
	}
	if name != "" && name != "-" && name[0] != '<' && name != "stdin" && name != "stdout" && name != "stderr" {
		clean := filepath.Clean(name)
		if abs, err := filepath.Abs(clean); err == nil {
			name = abs
		} else {
			name = clean
		}
	}
	raw, err := vfs.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}
	// Logical access derivation
	allowRead := true
	allowWrite := false
	if flag&os.O_WRONLY != 0 {
		allowRead = false
		allowWrite = true
	} else if flag&os.O_RDWR != 0 {
		allowRead = true
		allowWrite = true
	}
	// Determine base File
	// raw is io.ReadWriteCloser; wrap if needed for restriction
	if !(allowRead && allowWrite) {
		var base File
		switch v := raw.(type) {
		case *VirtualFile:
			base = v
		case *os.File:
			base = &StdFile{File: v, name: name}
		case File:
			base = v
		default:
			return nil, fmt.Errorf("unsupported type for restriction: %T", raw)
		}
		return &AccessModeFile{base: base, allowRead: allowRead, allowWrite: allowWrite, name: name}, nil
	}
	return raw, nil
}

// RegisterInputOutput registers input/output files based on execution level
// For top-level: -i, -o are real files (LLM hints + VFS read/write permission)
// For internal: -i, -o are temp files (internal context)
func (vfs *VirtualFS) RegisterInputOutput(name string, flag int, perm os.FileMode, isInput bool) (io.ReadWriteCloser, error) {
	if vfs.isTopLevel {
		// Top-level: treat as real file
		return vfs.RegisterRealFile(name, flag, perm)
	} else {
		// Internal level: create temp file with logical name
		if isInput {
			// For input, try to find existing file first
			if file, err := vfs.openFile(name, flag, perm); err == nil {
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
func (vfs *VirtualFS) IsTopLevel() bool {
	vfs.mutex.RLock()
	defer vfs.mutex.RUnlock()
	return vfs.isTopLevel
}

// AllowRealFile adds a real file to the allowed list (for top-level -i/-o)
func (vfs *VirtualFS) AllowRealFile(name string) {
	vfs.mutex.Lock()
	defer vfs.mutex.Unlock()

	vfs.allowedRealFiles[name] = true
}

// IsRealFileAllowed checks if a real file is in the allowed list
func (vfs *VirtualFS) IsRealFileAllowed(name string) bool {
	vfs.mutex.RLock()
	defer vfs.mutex.RUnlock()

	allowed := vfs.allowedRealFiles[name]
	return allowed
}

// RegisterRealFile registers a real filesystem file (for -i, -o command line options)
func (vfs *VirtualFS) RegisterRealFile(name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
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
	rawFile, err := os.OpenFile(name, flag, perm)
	if err != nil {
		return nil, fmt.Errorf("failed to open real file %s: %w", name, err)
	}
	// (debug logging removed)

	// Register in VFS as real file
	fd := vfs.nextFD
	vfs.nextFD++

	entry := &VFSEntry{
		Name:     name,
		FD:       fd,
		Type:     FileTypeRealFile,
		File:     rawFile, // os.File already implements File interface
		Consumed: false,
	}

	vfs.nameToFD[name] = fd
	vfs.fdToName[fd] = name
	vfs.entries[fd] = entry

	// Add to allowed list if top-level (for inheritance)
	if vfs.isTopLevel {
		vfs.allowedRealFiles[name] = true
	}

	return rawFile, nil
}

// IsRealFile checks if a file is registered as a real file
func (vfs *VirtualFS) IsRealFile(name string) bool {
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
func (vfs *VirtualFS) GetRealFiles() []string {
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
func (vfs *VirtualFS) InheritAllowedFiles(parentVFS *VirtualFS) {
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
func (vfs *VirtualFS) GetAllowedRealFiles() []string {
	vfs.mutex.RLock()
	defer vfs.mutex.RUnlock()

	files := make([]string, 0, len(vfs.allowedRealFiles))
	for filename := range vfs.allowedRealFiles {
		files = append(files, filename)
	}
	return files
}

// MarkConsumed marks a temp file as consumed
func (vfs *VirtualFS) MarkConsumed(name string) {
	vfs.mutex.Lock()
	defer vfs.mutex.Unlock()

	if fd, exists := vfs.nameToFD[name]; exists {
		if entry, exists := vfs.entries[fd]; exists && (entry.Type == FileTypeTempFile || entry.Type == FileTypeVirtual) {
			entry.Consumed = true
		}
	}
}

// ListEntries returns all VFS entries for debugging
func (vfs *VirtualFS) ListEntries() map[int]*VFSEntry {
	vfs.mutex.RLock()
	defer vfs.mutex.RUnlock()

	result := make(map[int]*VFSEntry)
	for fd, entry := range vfs.entries {
		result[fd] = entry
	}
	return result
}

// CreateTempFile creates a temporary file using O_TMPFILE for the given logical name
func (vfs *VirtualFS) CreateTempFile(logicalName string) (io.ReadWriteCloser, error) {
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
func (vfs *VirtualFS) ResolvePath(logicalName string) (string, bool) {
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
func (vfs *VirtualFS) Cleanup() {
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
func (vfs *VirtualFS) CreateTemp(pattern string) (io.ReadWriteCloser, string, error) {
	file, err := vfs.CreateTempFile(pattern)
	if err != nil {
		return nil, "", err
	}

	// Get file name from the underlying os.File
	if osFile, ok := file.(*os.File); ok {
		return file, osFile.Name(), nil
	}

	// Fallback to pattern if name cannot be determined
	return file, pattern, nil
}

// Remove removes a file (implements VirtualFileSystem interface)
func (vfs *VirtualFS) Remove(name string) error {
	return vfs.RemoveFile(name)
}

// RemoveFile removes a file from VFS
func (vfs *VirtualFS) RemoveFile(name string) error {
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
func (vfs *VirtualFS) ListFiles() []string {
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

// OpenForRead opens a file for reading
func (vfs *VirtualFS) OpenForRead(name string, allowReal bool) (interface{}, error) {
	// Use OpenFileSession so virtualMode + injection rules are enforced uniformly
	file, err := vfs.OpenFileSession(name, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// OpenForWrite opens a file for writing
func (vfs *VirtualFS) OpenForWrite(name string, append bool, allowReal bool) (interface{}, error) {
	flag := os.O_WRONLY | os.O_CREATE
	if append {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}
	file, err := vfs.OpenFileSession(name, flag, 0644)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// CreatePipe creates a pipe for reading and writing
func (vfs *VirtualFS) CreatePipe() (io.ReadCloser, io.WriteCloser, error) {
	// Create a unique name for the pipe
	name := fmt.Sprintf("<pipe-%d>", vfs.nextFD)

	// Create a virtual file for the pipe
	pipeFile := &VirtualFile{
		name: name,
		data: []byte{},
		flag: os.O_RDWR,
		perm: 0644,
	}

	// Register in VFS
	vfs.mutex.Lock()
	defer vfs.mutex.Unlock()

	fd := vfs.nextFD
	vfs.nextFD++

	entry := &VFSEntry{
		Name:     name,
		FD:       fd,
		Type:     FileTypeVirtual,
		File:     pipeFile,
		Consumed: false,
	}

	vfs.nameToFD[name] = fd
	vfs.fdToName[fd] = name
	vfs.entries[fd] = entry

	// Return the same file as both reader and writer
	// Since VirtualFile implements both io.ReadCloser and io.WriteCloser
	return pipeFile, pipeFile, nil
}

// =====================================
// VFS Client Layer (4-Layer Architecture)
// =====================================

// VFSClient provides OS-compatible file operations for LLMSH
// This is the VFS Client for LLMSH in the 4-layer architecture
type VFSClient struct {
	server     *VirtualFS
	isInternal bool
}

// NewVFSClient creates a new VFS client for LLMSH
func NewVFSClient(server *VirtualFS, isInternal bool) *VFSClient {
	return &VFSClient{server: server, isInternal: isInternal}
}

// OpenFile provides OS-compatible file opening interface
func (c *VFSClient) OpenFile(name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	return c.server.OpenFileSession(name, flag, perm)
}

// Create creates or truncates the named file
func (c *VFSClient) Create(name string) (io.ReadWriteCloser, error) {
	return c.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}

// CreateTemp creates a temporary file
func (c *VFSClient) CreateTemp(pattern string) (io.ReadWriteCloser, string, error) {
	return c.server.CreateTemp(pattern)
}

// Remove removes a file
func (c *VFSClient) Remove(name string) error {
	return c.server.Remove(name)
}

// LLMChat removed with fsproxy

// readProxyResponse removed with fsproxy

// LLMCMDVFSClient extends VFSClient with LLM token quota management for LLMCMD
// This is the VFS Client for LLMCMD in the 4-layer architecture
// Quota management is for OpenAI API tokens, not file operations
type LLMCMDVFSClient struct {
	*VFSClient
}

// NewLLMCMDVFSClient removed with fsproxy; use VFSClient directly

// LLMQuota gets current LLM token quota status via LLM_QUOTA command
// LLMQuota removed with fsproxy

// ProxyResponse represents a response from fsproxy protocol
// ProxyResponse removed with fsproxy
