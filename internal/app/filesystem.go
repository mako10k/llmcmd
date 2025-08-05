package app

import (
	"io"
	"os"
)

// FileSystem represents a virtual file system interface compatible with os package
type FileSystem interface {
	// OpenFile opens the named file with specified flag (O_RDONLY etc.) and perm.
	// Compatible with os.OpenFile
	OpenFile(name string, flag int, perm os.FileMode) (File, error)

	// Create creates or truncates the named file.
	// Compatible with os.Create
	Create(name string) (File, error)

	// CreateTemp creates a new temporary file in the directory dir
	// Compatible with os.CreateTemp
	CreateTemp(dir, pattern string) (File, error)

	// Remove removes the named file or directory.
	// Compatible with os.Remove
	Remove(name string) error
}

// File represents a file interface compatible with os.File
type File interface {
	io.ReadWriteCloser

	// Name returns the name of the file
	Name() string

	// Stat returns a FileInfo describing the named file
	Stat() (os.FileInfo, error)

	// Sync commits the current contents of the file to stable storage
	Sync() error

	// Truncate changes the size of the file
	Truncate(size int64) error

	// Seek sets the offset for the next Read or Write on file
	Seek(offset int64, whence int) (int64, error)
}

// RealFileSystem implements FileSystem using real os calls
type RealFileSystem struct{}

func (rfs *RealFileSystem) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	return os.OpenFile(name, flag, perm)
}

func (rfs *RealFileSystem) Create(name string) (File, error) {
	return os.Create(name)
}

func (rfs *RealFileSystem) CreateTemp(dir, pattern string) (File, error) {
	return os.CreateTemp(dir, pattern)
}

func (rfs *RealFileSystem) Remove(name string) error {
	return os.Remove(name)
}
