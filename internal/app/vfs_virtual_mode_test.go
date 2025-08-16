package app

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestVirtualModeInjectedReal ensures that in virtual mode only injected (-i/-o) files become real.
func TestVirtualModeInjectedReal(t *testing.T) {
	dir := t.TempDir()
	injectedPath := filepath.Join(dir, "injected.txt")
	if err := os.WriteFile(injectedPath, []byte("HELLO"), 0644); err != nil {
		t.Fatalf("failed to create injected file: %v", err)
	}

	vfs := VFSWithOptions(true, true, []string{injectedPath})

	f, err := vfs.OpenFileSession(injectedPath, os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("open injected real: %v", err)
	}
	// Expect this to be a real file (IsRealFile should be true)
	if !vfs.IsRealFile(injectedPath) {
		t.Fatalf("expected injected path to be registered as real file")
	}
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read injected real: %v", err)
	}
	if string(data) != "HELLO" {
		t.Fatalf("unexpected content: %q", data)
	}
}

// TestVirtualModeVirtualization ensures non-injected real paths are virtualized with no OS read.
func TestVirtualModeVirtualization(t *testing.T) {
	dir := t.TempDir()
	realPath := filepath.Join(dir, "real.txt")
	if err := os.WriteFile(realPath, []byte("SECRET"), 0644); err != nil {
		t.Fatalf("failed to create real file: %v", err)
	}

	vfs := VFSWithOptions(true, true, nil) // virtual mode, no injections

	f, err := vfs.OpenFileSession(realPath, os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("open virtualized: %v", err)
	}

	// Must NOT be registered as real file
	if vfs.IsRealFile(realPath) {
		t.Fatalf("virtualized path should not be marked real")
	}

	// Read should be empty because we created an empty virtual file, not the underlying file content
	data, err := io.ReadAll(f)
	if err != nil && err != io.EOF {
		t.Fatalf("read virtualized: %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("expected empty virtual content, got %q", data)
	}

	// Verify underlying file still has content (sanity check)
	diskData, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatalf("read underlying real file: %v", err)
	}
	if string(diskData) != "SECRET" {
		t.Fatalf("unexpected underlying content: %q", diskData)
	}
}

// TestInternalUnauthorizedRealVirtualized ensures internal context virtualizes non-injected real paths instead of failing.
func TestInternalUnauthorizedRealVirtualized(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "deny.txt")
	if err := os.WriteFile(target, []byte("DATA"), 0644); err != nil {
		t.Fatalf("create file: %v", err)
	}

	internalVFS := VFSWithOptions(false, false, nil)
	f, err := internalVFS.OpenFileSession(target, os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("internal open should virtualize, got error: %v", err)
	}
	if internalVFS.IsRealFile(target) {
		t.Fatalf("internal unauthorized path should not be real")
	}
	data2, _ := io.ReadAll(f)
	if len(data2) != 0 {
		t.Fatalf("expected empty virtualized content, got %q", data2)
	}
}

// TestAllowedInheritance ensures allowed real files are inheritable by internal contexts.
func TestAllowedInheritance(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "parent.txt")
	if err := os.WriteFile(path, []byte("PARENT"), 0644); err != nil {
		t.Fatalf("create parent file: %v", err)
	}

	top := VFSWithOptions(true, false, nil)
	if _, err := top.OpenFileSession(path, os.O_RDONLY, 0); err != nil {
		t.Fatalf("top open real: %v", err)
	}
	if !top.IsRealFile(path) {
		t.Fatalf("expected top to register real file")
	}

	child := VFSWithOptions(false, false, nil)
	child.InheritAllowedFiles(top)

	f, err := child.OpenFileSession(path, os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("child open inherited real: %v", err)
	}
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read child real: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("expected non-empty content from inherited real file")
	}
}
