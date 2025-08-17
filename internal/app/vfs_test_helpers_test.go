package app

import (
	"strings"
	"testing"
)

// openWriteAndCheck opens a file via adapter, writes data, and asserts success.
func openWriteAndCheck(t *testing.T, adapter *VFSFSProxyAdapter, path string, data []byte) {
	t.Helper()
	file, err := adapter.OpenFile(path, 0x40|0x1, 0644) // os.O_CREATE|os.O_WRONLY without importing os
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}
	defer file.Close()

	if len(data) > 0 {
		if n, err := file.Write(data); err != nil {
			t.Fatalf("Write failed: %v", err)
		} else if n != len(data) {
			t.Errorf("Expected to write %d bytes, got %d", len(data), n)
		}
	}
}

// createTempWriteAndCheck creates a temp file and optionally writes data; returns filename
func createTempWriteAndCheck(t *testing.T, adapter *VFSFSProxyAdapter, pattern string, data []byte) string {
	t.Helper()
	file, filename, err := adapter.CreateTemp(pattern)
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	defer file.Close()

	if pattern != "" && !strings.Contains(filename, pattern) {
		t.Errorf("Expected filename to contain '%s', got: %s", pattern, filename)
	}

	if len(data) > 0 {
		if _, err := file.Write(data); err != nil {
			t.Fatalf("Write to temp file failed: %v", err)
		}
	}
	return filename
}

// listFilesContains asserts that adapter.ListFiles contains any of the given substrings or names.
func listFilesContains(t *testing.T, adapter *VFSFSProxyAdapter, anyOf ...string) {
	t.Helper()
	files := adapter.ListFiles()
	if len(anyOf) == 0 {
		if len(files) == 0 {
			t.Error("Expected some files to be listed")
		}
		return
	}
	found := false
	for _, f := range files {
		for _, want := range anyOf {
			if strings.Contains(f, want) || f == want {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Errorf("Expected one of %v to be in file list", anyOf)
	}
}
