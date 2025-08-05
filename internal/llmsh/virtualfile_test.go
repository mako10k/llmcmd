package llmsh

import (
	"bytes"
	"testing"
)

func TestVirtualFileBasics(t *testing.T) {
	vf := NewVirtualFile("test.txt")

	// Test write
	content := "Hello, Virtual File!"
	n, err := vf.Write([]byte(content))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(content) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(content), n)
	}

	// Test read using GetReader
	reader := vf.GetReader()
	var buf bytes.Buffer
	_, err = buf.ReadFrom(reader)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if buf.String() != content {
		t.Errorf("Expected content '%s', got '%s'", content, buf.String())
	}

	// Test direct Read method
	p := make([]byte, len(content))
	n, err = vf.Read(p)
	if err != nil {
		t.Fatalf("Direct read failed: %v", err)
	}

	readContent := string(p[:n])
	if readContent != content {
		t.Errorf("Expected direct read content '%s', got '%s'", content, readContent)
	}
}
