package builtin

import (
	"strings"
	"testing"
)

func TestCat(t *testing.T) {
	input := strings.NewReader("Hello\nWorld\n")
	var output strings.Builder
	
	err := Cat([]string{}, input, &output)
	if err != nil {
		t.Errorf("Cat failed: %v", err)
	}
	
	expected := "Hello\nWorld\n"
	if output.String() != expected {
		t.Errorf("Cat output = %q, want %q", output.String(), expected)
	}
}

func TestSort(t *testing.T) {
	input := strings.NewReader("banana\napple\ncherry\n")
	var output strings.Builder
	
	err := Sort([]string{}, input, &output)
	if err != nil {
		t.Errorf("Sort failed: %v", err)
	}
	
	expected := "apple\nbanana\ncherry\n"
	if output.String() != expected {
		t.Errorf("Sort output = %q, want %q", output.String(), expected)
	}
}

func TestGrep(t *testing.T) {
	input := strings.NewReader("apple\nbanana\ncherry\napricot\n")
	var output strings.Builder
	
	err := Grep([]string{"ap"}, input, &output)
	if err != nil {
		t.Errorf("Grep failed: %v", err)
	}
	
	expected := "apple\napricot\n"
	if output.String() != expected {
		t.Errorf("Grep output = %q, want %q", output.String(), expected)
	}
}

func TestWc(t *testing.T) {
	input := strings.NewReader("line one\nline two\nline three\n")
	var output strings.Builder
	
	err := Wc([]string{}, input, &output)
	if err != nil {
		t.Errorf("Wc failed: %v", err)
	}
	
	// Should show: lines words chars
	result := strings.TrimSpace(output.String())
	parts := strings.Fields(result)
	
	if len(parts) != 3 {
		t.Errorf("Wc should output 3 numbers, got %d: %s", len(parts), result)
	}
	
	if parts[0] != "3" { // 3 lines
		t.Errorf("Wc lines = %s, want 3", parts[0])
	}
	if parts[1] != "6" { // 6 words
		t.Errorf("Wc words = %s, want 6", parts[1])
	}
}
