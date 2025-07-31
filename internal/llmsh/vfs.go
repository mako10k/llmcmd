package llmsh

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"
)

// VirtualFileSystem manages virtual files and pipes for llmsh
type VirtualFileSystem struct {
	mu sync.RWMutex
	
	// Virtual files (temporary named pipes)
	files map[string]*VirtualFile
	
	// Real files (stdin, stdout, stderr, input/output files)
	realFiles map[string]io.ReadWriteCloser
	
	// Allowed file access
	inputFile  string
	outputFile string
}

// VirtualFile represents a virtual file in memory
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

// Read reads from the virtual file
func (vf *VirtualFile) Read(p []byte) (n int, err error) {
	vf.mu.RLock()
	defer vf.mu.RUnlock()
	
	if vf.closed {
		return 0, fmt.Errorf("file %s is closed", vf.name)
	}
	
	return vf.buffer.Read(p)
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

// Close closes the virtual file
func (vf *VirtualFile) Close() error {
	vf.mu.Lock()
	defer vf.mu.Unlock()
	
	vf.closed = true
	return nil
}

// NewVirtualFileSystem creates a new VFS
func NewVirtualFileSystem(inputFile, outputFile string) *VirtualFileSystem {
	vfs := &VirtualFileSystem{
		files:      make(map[string]*VirtualFile),
		realFiles:  make(map[string]io.ReadWriteCloser),
		inputFile:  inputFile,
		outputFile: outputFile,
	}
	
	// Set up real files
	vfs.realFiles["stdin"] = os.Stdin
	vfs.realFiles["stdout"] = os.Stdout
	vfs.realFiles["stderr"] = os.Stderr
	
	return vfs
}

// OpenForRead opens a file for reading
func (vfs *VirtualFileSystem) OpenForRead(filename string) (io.ReadCloser, error) {
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()
	
	// Check for real files first
	if filename == "stdin" || filename == vfs.inputFile {
		if realFile, exists := vfs.realFiles[filename]; exists {
			return realFile.(io.ReadCloser), nil
		}
		
		// If it's the input file, try to open it
		if filename == vfs.inputFile && vfs.inputFile != "" {
			file, err := os.Open(vfs.inputFile)
			if err != nil {
				return nil, fmt.Errorf("cannot open input file %s: %v", vfs.inputFile, err)
			}
			vfs.realFiles[filename] = file
			return file, nil
		}
	}
	
	// Check for virtual files
	if vfile, exists := vfs.files[filename]; exists {
		return vfile, nil
	}
	
	return nil, fmt.Errorf("file not found: %s", filename)
}

// OpenForWrite opens a file for writing
func (vfs *VirtualFileSystem) OpenForWrite(filename string, append bool) (io.WriteCloser, error) {
	vfs.mu.Lock()
	defer vfs.mu.Unlock()
	
	// Check for real files first
	if filename == "stdout" || filename == "stderr" || filename == vfs.outputFile {
		if realFile, exists := vfs.realFiles[filename]; exists {
			return realFile.(io.WriteCloser), nil
		}
		
		// If it's the output file, try to create/open it
		if filename == vfs.outputFile && vfs.outputFile != "" {
			var file *os.File
			var err error
			
			if append {
				file, err = os.OpenFile(vfs.outputFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			} else {
				file, err = os.Create(vfs.outputFile)
			}
			
			if err != nil {
				return nil, fmt.Errorf("cannot create output file %s: %v", vfs.outputFile, err)
			}
			
			vfs.realFiles[filename] = file
			return file, nil
		}
	}
	
	// Create or get virtual file
	vfile, exists := vfs.files[filename]
	if !exists {
		vfile = NewVirtualFile(filename)
		vfs.files[filename] = vfile
	} else if !append {
		// Truncate if not appending
		vfile.buffer.Reset()
	}
	
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
