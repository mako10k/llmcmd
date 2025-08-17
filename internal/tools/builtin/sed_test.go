package builtin

import (
    "bytes"
    "fmt"
    "io"
    "os"
    "strings"
    "testing"
)

func TestSedBasicFirstOnly(t *testing.T) {
    in := strings.NewReader("foo bar foo\nfoo\n")
    var out bytes.Buffer
    if err := Sed([]string{"s/foo/FOO/"}, in, &out); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    got := out.String()
    want := "FOO bar foo\nFOO\n"
    if got != want {
        t.Fatalf("got %q, want %q", got, want)
    }
}

func TestSedGlobal(t *testing.T) {
    in := strings.NewReader("foo bar foo\nFOO foo\n")
    var out bytes.Buffer
    if err := Sed([]string{"s/foo/x/g"}, in, &out); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    got := out.String()
    want := "x bar x\nFOO x\n"
    if got != want {
        t.Fatalf("got %q, want %q", got, want)
    }
}

func TestSedIgnoreCase(t *testing.T) {
    in := strings.NewReader("Error error ERROR\n")
    var out bytes.Buffer
    if err := Sed([]string{"s/error/ok/i"}, in, &out); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    got := out.String()
    want := "ok error ERROR\n"
    if got != want {
        t.Fatalf("got %q, want %q", got, want)
    }
}

func TestSedIgnoreCaseGlobal(t *testing.T) {
    in := strings.NewReader("Error error ERROR\n")
    var out bytes.Buffer
    if err := Sed([]string{"s/error/ok/ig"}, in, &out); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    got := out.String()
    want := "ok ok ok\n"
    if got != want {
        t.Fatalf("got %q, want %q", got, want)
    }
}

func TestSedErrors(t *testing.T) {
    var out bytes.Buffer
    if err := Sed([]string{}, strings.NewReader(""), &out); err == nil {
        t.Fatalf("expected error for missing expr")
    }
    if err := Sed([]string{"x/foo/bar/"}, strings.NewReader(""), &out); err == nil {
        t.Fatalf("expected error for unsupported command")
    }
    if err := Sed([]string{"s/[foo/bar/"}, strings.NewReader(""), &out); err == nil {
        t.Fatalf("expected error for invalid regex")
    }
}

func TestSedVariableDelimiterAndEscapes(t *testing.T) {
    in := strings.NewReader("a/b c/d\n")
    var out bytes.Buffer
    if err := Sed([]string{"s#/#:#g"}, in, &out); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if got, want := out.String(), "a:b c:d\n"; got != want {
        t.Fatalf("got %q, want %q", got, want)
    }
}

func TestSedBackreferencesAndAmpersand(t *testing.T) {
    in := strings.NewReader("xy xy\n")
    var out bytes.Buffer
    if err := Sed([]string{"s#(x)(y)#&-\\1\\2#g"}, in, &out); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if got, want := out.String(), "xy-xy xy-xy\n"; got != want {
        t.Fatalf("got %q, want %q", got, want)
    }
}

func TestSedNoAutoPrintWithP(t *testing.T) {
    in := strings.NewReader("foo\nbar\n")
    var out bytes.Buffer
    if err := Sed([]string{"-n", "s/foo/bar/p"}, in, &out); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if got, want := out.String(), "bar\n"; got != want {
        t.Fatalf("got %q, want %q", got, want)
    }
}

func TestSedWithFilesUsesStdinWhenNoFiles(t *testing.T) {
    in := strings.NewReader("foo\n")
    var out bytes.Buffer
    if err := Sed([]string{"s/foo/bar/"}, in, &out); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if got, want := out.String(), "bar\n"; got != want {
        t.Fatalf("got %q, want %q", got, want)
    }
}

// Minimal VFS adapter for file-based test
type testVFS struct{}

func (testVFS) OpenFileSession(name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
    // For testing, always return a simple in-memory file
    return nopCloser{Reader: strings.NewReader("foo\n")}, nil
}

type nopCloser struct{ io.Reader }

func (n nopCloser) Read(p []byte) (int, error) { return n.Reader.Read(p) }
func (n nopCloser) Write([]byte) (int, error)  { return 0, fmt.Errorf("write not supported") }
func (n nopCloser) Close() error               { return nil }

func TestSedFileArgument(t *testing.T) {
    // Inject test VFS
    old := currentVFS
    SetVFS(testVFS{})
    defer SetVFS(old)

    var out bytes.Buffer
    if err := Sed([]string{"s/foo/bar/", "test_sed.txt"}, strings.NewReader("IGNORED"), &out); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if got, want := out.String(), "bar\n"; got != want {
        t.Fatalf("got %q, want %q", got, want)
    }
}
